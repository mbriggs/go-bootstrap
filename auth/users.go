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
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mbriggs/pgsql"
	"golang.org/x/crypto/bcrypt"

	"github.com/mbriggs/go-bootstrap/db"
)

// bcryptCost is intentionally above the default (10). The signin path is
// rare; users tolerate a 100ms hash check.
const bcryptCost = 12

// Password policy: long enough to resist online guessing, capped at
// bcrypt's 72-byte input limit so over-long passwords surface as a domain
// error instead of x/crypto's opaque one at hash time.
const (
	PasswordMinLength = 8
	passwordMaxBytes  = 72
)

const emailUniqueIndex = "users_email_lower_unique"

// RoleAdmin is the only role the bootstrap ships; projects add their own
// role constants alongside it.
const RoleAdmin = "admin"

var (
	ErrEmailRequired    = errors.New("auth: email is required")
	ErrPasswordRequired = errors.New("auth: password is required")
	ErrPasswordTooShort = errors.New("auth: password is shorter than the minimum length")
	ErrPasswordTooLong  = errors.New("auth: password is longer than 72 bytes (bcrypt limit)")

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

// CreateInput is the input for Create. Name is optional; falls back to the
// local part of the email.
type CreateInput struct {
	Email    string
	Password string
	Name     string
	Roles    []string
}

// CreateTx inserts a user, hashing the password with bcrypt. Surfaces
// ErrEmailTaken when the lower(email) unique index fires.
func CreateTx(ctx context.Context, tx db.Queryable, in CreateInput) (User, error) {
	email := strings.TrimSpace(in.Email)
	if email == "" {
		return User{}, ErrEmailRequired
	}
	if err := ValidatePassword(in.Password); err != nil {
		return User{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcryptCost)
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

	user := User{Email: email, PasswordHash: string(hash), Name: name, Roles: in.Roles}

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
// password so callers can't probe the user list.
func AuthenticateTx(ctx context.Context, tx db.Queryable, email, password string) (User, error) {
	user, err := ByEmailTx(ctx, tx, email)
	if errors.Is(err, db.ErrNotFound) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
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
// PasswordMinLength characters, at most 72 bytes. Only password-setting
// paths call it — authentication accepts whatever was stored.
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
