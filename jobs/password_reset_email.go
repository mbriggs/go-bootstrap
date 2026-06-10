package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/riverqueue/river"

	"github.com/mbriggs/go-bootstrap/auth"
	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/mailer"
)

// PasswordResetEmailArgs identifies the account to mail. Deliberately no
// token: the worker mints it at send time, so the cleartext never persists
// anywhere — not in river_job args, not in the table (which only holds the
// hash; see auth/reset.go).
type PasswordResetEmailArgs struct {
	Email string `json:"email"`
}

func (PasswordResetEmailArgs) Kind() string { return "password_reset_email" }

type passwordResetEmailWorker struct {
	river.WorkerDefaults[PasswordResetEmailArgs]
	baseURL string
}

// Work mints a fresh token per attempt — a retry after a failed send
// issues a new link; earlier ones die on their TTL and all retire on use.
func (w *passwordResetEmailWorker) Work(ctx context.Context, job *river.Job[PasswordResetEmailArgs]) error {
	token, _, err := auth.CreatePasswordReset(ctx, job.Args.Email)
	if errors.Is(err, db.ErrNotFound) {
		logger.Info("skipping reset email; account vanished since request", "email", job.Args.Email)
		return nil
	}
	if err != nil {
		return fmt.Errorf("creating reset token: %w", err)
	}

	link := w.baseURL + "/password-reset/confirm?token=" + token

	return mailer.Send(ctx, mailer.Message{
		To:      job.Args.Email,
		Subject: "Reset your password",
		Body: fmt.Sprintf(
			"A password reset was requested for this address.\n\n"+
				"Reset your password: %s\n\n"+
				"The link expires in an hour. If you didn't ask for this, ignore this email.",
			link,
		),
	})
}
