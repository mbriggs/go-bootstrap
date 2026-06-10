// Password reset tokens. The cleartext token only ever exists in the
// emailed link; the table stores its sha256, so a database read can't mint
// working reset links. Tokens are single-use and short-lived, and a
// successful reset retires every outstanding token for that user.

package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/pgsql"
)

// ResetTokenTTL is how long an emailed reset link stays valid.
const ResetTokenTTL = time.Hour

// ErrResetTokenInvalid covers unknown, expired, and already-used tokens —
// one sentinel, so responses can't be used to probe which case it was.
var ErrResetTokenInvalid = errors.New("auth: reset token invalid")

const resetTokenColumns = "id, user_id, token_hash, expires_at, used_at, created_at" //nolint:gosec // column list, not a credential

// CreatePasswordResetTx issues a reset token for the user behind email and
// returns the cleartext token (for the emailed link) with the user. Unknown
// email surfaces as db.ErrNotFound — callers must swallow it into the same
// response as success so the endpoint can't probe the user table.
func CreatePasswordResetTx(ctx context.Context, tx db.Queryable, email string) (string, User, error) {
	user, err := ByEmailTx(ctx, tx, email)
	if err != nil {
		return "", User{}, err
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", User{}, fmt.Errorf("generating reset token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	row := PasswordResetToken{
		UserID:    user.ID,
		TokenHash: hashResetToken(token),
		ExpiresAt: time.Now().Add(ResetTokenTTL),
	}

	_, err = db.InsertTx[PasswordResetToken](ctx, tx,
		pgsql.Insert("password_reset_tokens").Data(row.ToRowMap()).Returning(resetTokenColumns))
	if err != nil {
		return "", User{}, fmt.Errorf("inserting reset token: %w", err)
	}

	return token, user, nil
}

// CheckResetTokenTx reports whether token is live (exists, unused,
// unexpired) — for showing the new-password form before the user types
// anything. Returns ErrResetTokenInvalid otherwise.
func CheckResetTokenTx(ctx context.Context, tx db.Queryable, token string) error {
	_, err := liveResetTokenTx(ctx, tx, token)
	return err
}

// ResetPasswordTx consumes token and sets the user's password. Returns
// ErrResetTokenInvalid for unknown/expired/used tokens and
// ErrPasswordRequired for an empty password. Success retires all of the
// user's outstanding tokens, not just this one.
func ResetPasswordTx(ctx context.Context, tx db.Queryable, token, password string) (User, error) {
	if password == "" {
		return User{}, ErrPasswordRequired
	}

	row, err := liveResetTokenTx(ctx, tx, token)
	if err != nil {
		return User{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return User{}, fmt.Errorf("hashing password: %w", err)
	}

	_, err = db.UpdateTx(ctx, tx, pgsql.Update("users").
		Setf("password_hash = ?, updated_at = now()", string(hash)).
		Where("id = ?", row.UserID))
	if err != nil {
		return User{}, fmt.Errorf("updating password: %w", err)
	}

	_, err = db.UpdateTx(ctx, tx, pgsql.Update("password_reset_tokens").
		Setf("used_at = now()").
		Where("user_id = ? AND used_at IS NULL", row.UserID))
	if err != nil {
		return User{}, fmt.Errorf("retiring reset tokens: %w", err)
	}

	return ByIDTx(ctx, tx, row.UserID)
}

// liveResetTokenTx fetches the unused, unexpired row for token, mapping
// not-found to ErrResetTokenInvalid.
func liveResetTokenTx(ctx context.Context, tx db.Queryable, token string) (PasswordResetToken, error) {
	row, err := db.FindTx[PasswordResetToken](ctx, tx,
		"SELECT "+resetTokenColumns+
			" FROM password_reset_tokens WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()",
		hashResetToken(token))
	if errors.Is(err, db.ErrNotFound) {
		return PasswordResetToken{}, ErrResetTokenInvalid
	}
	if err != nil {
		return PasswordResetToken{}, err
	}

	return row, nil
}

func hashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
