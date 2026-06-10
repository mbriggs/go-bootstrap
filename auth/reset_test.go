package auth_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/mbriggs/go-bootstrap/auth"
	"github.com/mbriggs/go-bootstrap/db"
)

func expireResetTokens(t *testing.T, userID int64) {
	t.Helper()

	err := db.ExecInTx(t.Context(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(t.Context(),
			"UPDATE password_reset_tokens SET expires_at = now() - interval '1 minute' WHERE user_id = $1",
			userID); err != nil {
			return fmt.Errorf("expiring tokens for user %d: %w", userID, err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expire tokens: %v", err)
	}
}

func TestResetPasswordConsumesToken(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	user, err := auth.Make(ctx)
	if err != nil {
		t.Fatalf("make user: %v", err)
	}

	token, tokenUser, err := auth.CreatePasswordReset(ctx, user.Email)
	if err != nil {
		t.Fatalf("create reset: %v", err)
	}
	if tokenUser.ID != user.ID || token == "" {
		t.Fatalf("create reset = (%q, user %d), want token for user %d", token, tokenUser.ID, user.ID)
	}

	if err := auth.CheckResetToken(ctx, token); err != nil {
		t.Fatalf("live token check = %v, want nil", err)
	}

	if _, err := auth.ResetPassword(ctx, token, "after-reset-pw"); err != nil {
		t.Fatalf("reset password: %v", err)
	}

	if _, err := auth.Authenticate(ctx, user.Email, "after-reset-pw"); err != nil {
		t.Fatalf("authenticate with new password: %v", err)
	}
	if _, err := auth.Authenticate(ctx, user.Email, auth.MakePassword); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("authenticate with old password = %v, want ErrInvalidCredentials", err)
	}

	// Single-use: the consumed token is dead.
	if err := auth.CheckResetToken(ctx, token); !errors.Is(err, auth.ErrResetTokenInvalid) {
		t.Fatalf("used token check = %v, want ErrResetTokenInvalid", err)
	}
}

func TestResetPasswordRetiresAllOutstandingTokens(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	user, err := auth.Make(ctx)
	if err != nil {
		t.Fatalf("make user: %v", err)
	}

	first, _, err := auth.CreatePasswordReset(ctx, user.Email)
	if err != nil {
		t.Fatalf("create first reset: %v", err)
	}
	second, _, err := auth.CreatePasswordReset(ctx, user.Email)
	if err != nil {
		t.Fatalf("create second reset: %v", err)
	}

	if _, err := auth.ResetPassword(ctx, first, "replacement-pw"); err != nil {
		t.Fatalf("reset with first token: %v", err)
	}

	// A successful reset retires the other outstanding token too.
	if _, err := auth.ResetPassword(ctx, second, "sneaky-pw"); !errors.Is(err, auth.ErrResetTokenInvalid) {
		t.Fatalf("reset with retired token = %v, want ErrResetTokenInvalid", err)
	}
}

func TestResetPasswordRejectsExpiredToken(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	user, err := auth.Make(ctx)
	if err != nil {
		t.Fatalf("make user: %v", err)
	}

	token, _, err := auth.CreatePasswordReset(ctx, user.Email)
	if err != nil {
		t.Fatalf("create reset: %v", err)
	}

	expireResetTokens(t, user.ID)

	if _, err := auth.ResetPassword(ctx, token, "replacement-pw"); !errors.Is(err, auth.ErrResetTokenInvalid) {
		t.Fatalf("reset with expired token = %v, want ErrResetTokenInvalid", err)
	}
}

func TestResetPasswordEnforcesPasswordPolicy(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	user, err := auth.Make(ctx)
	if err != nil {
		t.Fatalf("make user: %v", err)
	}

	token, _, err := auth.CreatePasswordReset(ctx, user.Email)
	if err != nil {
		t.Fatalf("create reset: %v", err)
	}

	if _, err := auth.ResetPassword(ctx, token, ""); !errors.Is(err, auth.ErrPasswordRequired) {
		t.Fatalf("reset with empty password = %v, want ErrPasswordRequired", err)
	}
	if _, err := auth.ResetPassword(ctx, token, "seven77"); !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Fatalf("reset with short password = %v, want ErrPasswordTooShort", err)
	}

	// The failed attempts must not burn the token.
	if err := auth.CheckResetToken(ctx, token); err != nil {
		t.Fatalf("token after rejected reset = %v, want still live", err)
	}

	if _, _, err := auth.CreatePasswordReset(ctx, "missing-reset@example.com"); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("create reset for unknown email = %v, want db.ErrNotFound", err)
	}
}
