// Package auth owns the users table and the password-credential flow that
// backs the scs session cookie. Sessions themselves live in alexedwards/scs
// with the pgxstore — this package only writes the user side.
//
// User.PasswordHash is on the model because the model mirrors the row; never
// serialize a User directly to a client (render fields explicitly, or shape
// a response DTO).
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mbriggs/pgsql"

	"github.com/mbriggs/go-bootstrap/db"
)

// hashParams are OWASP's recommended argon2id minimums (19 MiB, t=2, p=1).
// The signin path is rare; users tolerate the ~50ms hash check.
var hashParams = &argon2id.Params{
	Memory:      19 * 1024,
	Iterations:  2,
	Parallelism: 1,
	SaltLength:  16,
	KeyLength:   32,
}

// dummyHash absorbs a compare when the email doesn't exist, so unknown
// email and wrong password take the same time — the throttle key is
// per-email, so timing is the one enumeration probe it doesn't cover.
var dummyHash = func() string {
	hash, err := argon2id.CreateHash("decoy-for-timing-uniformity", hashParams)
	if err != nil {
		panic("auth: creating dummy hash: " + err.Error())
	}

	return hash
}()

// hashSlots caps concurrent argon2 work. Memory-hardness cuts both ways:
// each hash or compare holds ~19 MiB, the signin throttle keys per (IP,
// email), so an attacker rotating emails gets a fresh budget per request —
// without a cap, parallel signin POSTs are a cheap memory-amplification
// attack. 2×GOMAXPROCS bounds that at ~40 MiB per core while leaving
// headroom for legitimate bursts; excess callers queue until their ctx
// gives up.
var hashSlots = make(chan struct{}, 2*runtime.GOMAXPROCS(0))

func hashPassword(ctx context.Context, password string) (string, error) {
	select {
	case hashSlots <- struct{}{}:
		defer func() { <-hashSlots }()
	case <-ctx.Done():
		return "", fmt.Errorf("waiting for a hash slot: %w", ctx.Err())
	}

	hash, err := argon2id.CreateHash(password, hashParams)
	if err != nil {
		return "", fmt.Errorf("argon2id hash: %w", err)
	}

	return hash, nil
}

func comparePassword(ctx context.Context, password, hash string) (bool, error) {
	select {
	case hashSlots <- struct{}{}:
		defer func() { <-hashSlots }()
	case <-ctx.Done():
		return false, fmt.Errorf("waiting for a hash slot: %w", ctx.Err())
	}

	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, fmt.Errorf("argon2id compare: %w", err)
	}

	return match, nil
}

// Password policy: long enough to resist online guessing, capped so a
// client can't feed argon2 arbitrarily large inputs as a cheap DoS.
const (
	PasswordMinLength = 8
	passwordMaxBytes  = 512
)

const emailUniqueIndex = "users_email_lower_unique"

// RoleAdmin is the only role the bootstrap ships; projects add their own
// role constants alongside it.
const RoleAdmin = "admin"

var (
	ErrEmailRequired    = errors.New("auth: email is required")
	ErrPasswordRequired = errors.New("auth: password is required")
	ErrPasswordTooShort = errors.New("auth: password is shorter than the minimum length")
	ErrPasswordTooLong  = errors.New("auth: password is longer than the maximum length")

	// ErrInvalidCredentials is what callers branch on at the seam — never
	// distinguish "no such email" from "wrong password" at the wire so
	// credential stuffing can't probe the user table.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")

	// ErrEmailTaken surfaces from Create when the lower(email) unique index
	// fires.
	ErrEmailTaken = errors.New("auth: email already in use")
)

const userColumns = "id, email, password_hash, name, roles, created_at, updated_at"

func (user User) HasRole(role string) bool {
	return slices.Contains(user.Roles, role)
}

// PasswordEpoch identifies the current password generation. Sessions stamp
// it at signin and check it on every request, so changing the password
// invalidates every outstanding session — including one an attacker holds.
// It is a digest of the hash, not the hash itself, so session rows never
// carry crackable material.
func (user User) PasswordEpoch() string {
	sum := sha256.Sum256([]byte(user.PasswordHash))
	return hex.EncodeToString(sum[:])
}

// CreateInput is the input for Create. Name is optional; falls back to the
// local part of the email.
type CreateInput struct {
	Email    string
	Password string
	Name     string
	Roles    []string
}

// CreateTx inserts a user, hashing the password with argon2id. Surfaces
// ErrEmailTaken when the lower(email) unique index fires.
func CreateTx(ctx context.Context, tx db.Queryable, in CreateInput) (User, error) {
	email := strings.TrimSpace(in.Email)
	if email == "" {
		return User{}, ErrEmailRequired
	}
	if err := ValidatePassword(in.Password); err != nil {
		return User{}, err
	}

	hash, err := hashPassword(ctx, in.Password)
	if err != nil {
		return User{}, fmt.Errorf("hashing password: %w", err)
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = email
		if at := strings.IndexByte(email, '@'); at > 0 {
			name = email[:at]
		}
	}

	user := User{Email: email, PasswordHash: hash, Name: name, Roles: in.Roles}

	created, err := db.InsertTx[User](ctx, tx, pgsql.Insert("users").Data(user.ToRowMap()).Returning(userColumns))
	if err != nil {
		if isEmailUniqueViolation(err) {
			return User{}, fmt.Errorf("%w: %s", ErrEmailTaken, email)
		}
		return User{}, fmt.Errorf("inserting user: %w", err)
	}

	return created, nil
}

// AuthenticateTx verifies an email+password pair and returns the user on
// success. Returns ErrInvalidCredentials for both unknown email and bad
// password so callers can't probe the user list; unknown email still pays
// for a hash compare so the two failures are timing-uniform too.
func AuthenticateTx(ctx context.Context, tx db.Queryable, email, password string) (User, error) {
	user, err := ByEmailTx(ctx, tx, email)
	if errors.Is(err, db.ErrNotFound) {
		_, _ = comparePassword(ctx, password, dummyHash)
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}

	match, err := comparePassword(ctx, password, user.PasswordHash)
	if err != nil {
		return User{}, fmt.Errorf("comparing password hash: %w", err)
	}
	if !match {
		return User{}, ErrInvalidCredentials
	}

	return user, nil
}

// ByEmailTx returns the user matching email case-insensitively, or
// db.ErrNotFound.
func ByEmailTx(ctx context.Context, tx db.Queryable, email string) (User, error) {
	return db.FindExactlyOneTx[User](ctx, tx,
		"SELECT "+userColumns+" FROM users WHERE lower(email) = lower($1)",
		strings.TrimSpace(email))
}

// ByIDTx returns the user with the given id, or db.ErrNotFound.
func ByIDTx(ctx context.Context, tx db.Queryable, id int64) (User, error) {
	return db.FindExactlyOneTx[User](ctx, tx,
		"SELECT "+userColumns+" FROM users WHERE id = $1", id)
}

// ValidatePassword applies the password policy: required, at least
// PasswordMinLength characters, at most passwordMaxBytes. Only
// password-setting paths call it — authentication accepts whatever was
// stored.
func ValidatePassword(password string) error {
	switch {
	case password == "":
		return ErrPasswordRequired
	case utf8.RuneCountInString(password) < PasswordMinLength:
		return ErrPasswordTooShort
	case len(password) > passwordMaxBytes:
		return ErrPasswordTooLong
	}

	return nil
}

func isEmailUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == "23505" && pgErr.ConstraintName == emailUniqueIndex
}
