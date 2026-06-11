package web

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"

	"github.com/mbriggs/go-bootstrap/auth"
	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/jobs"
	"github.com/mbriggs/go-bootstrap/views"
)

func SigninForm(c *echo.Context) error {
	if CurrentUser(c) != nil {
		return SafeRedirect(c, "/")
	}

	return RenderPage(c, PageMeta{Title: "Sign in"}, views.SigninPage(""))
}

func SigninSubmit(c *echo.Context) error {
	if err := ParseForm(c); err != nil {
		return err
	}

	email := strings.TrimSpace(c.FormValue("email"))
	password := c.FormValue("password")
	ctx := c.Request().Context()

	throttleKey := c.RealIP() + "|" + strings.ToLower(email)
	blocked, err := ThrottleBlocked(ctx, throttleKey)
	if err != nil {
		return fmt.Errorf("checking signin throttle: %w", err)
	}
	if blocked {
		SetFlash(c, "error", "too many failed attempts — try again later")
		return RenderPage(c, PageMeta{Title: "Sign in"}, views.SigninPage(email))
	}

	user, err := auth.Authenticate(ctx, email, password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		if err := ThrottleFail(ctx, throttleKey); err != nil {
			return fmt.Errorf("recording failed signin: %w", err)
		}
		SetFlash(c, "error", "invalid email or password")
		return RenderPage(c, PageMeta{Title: "Sign in"}, views.SigninPage(email))
	}
	if err != nil {
		return fmt.Errorf("authenticating: %w", err)
	}
	// Best-effort: a failed cleanup only leaves stale failure rows that
	// expire with the window — not worth failing a valid signin over.
	if err := ThrottleReset(ctx, throttleKey); err != nil {
		logger.ErrorContext(ctx, "clearing signin throttle", "error", err)
	}

	// Rotate the session id on signin to defend against fixation.
	if err := Sessions.RenewToken(ctx); err != nil {
		return fmt.Errorf("renewing session: %w", err)
	}
	Sessions.Put(ctx, "user_id", user.ID)
	// LoadUser kills the session when this stops matching, so a password
	// change signs out every session that signed in before it.
	Sessions.Put(ctx, "password_epoch", user.PasswordEpoch())

	return SafeRedirect(c, Sessions.PopString(ctx, "after_signin"))
}

// PasswordResetRequestForm renders the "enter your email" form.
func PasswordResetRequestForm(c *echo.Context) error {
	return RenderPage(c, PageMeta{Title: "Reset password"}, views.PasswordResetRequestPage(""))
}

// PasswordResetRequest enqueues the reset email. The response never
// reveals whether the email exists, and every request counts against the
// throttle — this endpoint sends mail, so it throttles on attempts, not
// failures.
func PasswordResetRequest(c *echo.Context) error {
	if err := ParseForm(c); err != nil {
		return err
	}

	email := strings.TrimSpace(c.FormValue("email"))
	ctx := c.Request().Context()

	throttleKey := "reset|" + c.RealIP() + "|" + strings.ToLower(email)
	blocked, err := ThrottleBlocked(ctx, throttleKey)
	if err != nil {
		return fmt.Errorf("checking reset throttle: %w", err)
	}
	if blocked {
		SetFlash(c, "error", "too many reset requests — try again later")
		return RenderPage(c, PageMeta{Title: "Reset password"}, views.PasswordResetRequestPage(email))
	}
	// Every request counts, success or not — this endpoint sends mail.
	if err := ThrottleFail(ctx, throttleKey); err != nil {
		return fmt.Errorf("recording reset request: %w", err)
	}

	// Standalone enqueue: there is no accompanying state change — the
	// worker mints the token at send time. Unknown email enqueues nothing
	// and answers identically.
	user, err := auth.ByEmail(ctx, email)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return fmt.Errorf("looking up reset account: %w", err)
	}
	if err == nil {
		if _, err := jobs.InsertStandalone(ctx, jobs.PasswordResetEmailArgs{Email: user.Email}); err != nil {
			return fmt.Errorf("enqueueing reset email: %w", err)
		}
	}

	SetFlash(c, "ok", "if that email has an account, a reset link is on the way")
	return SafeRedirect(c, "/signin")
}

// PasswordResetConfirmForm checks the emailed token before showing the
// new-password form, so a dead link fails before the user types anything.
func PasswordResetConfirmForm(c *echo.Context) error {
	token := c.QueryParam("token")

	err := auth.CheckResetToken(c.Request().Context(), token)
	if errors.Is(err, auth.ErrResetTokenInvalid) {
		SetFlash(c, "error", "that reset link is no longer valid — request a new one")
		return SafeRedirect(c, "/password-reset")
	}
	if err != nil {
		return fmt.Errorf("checking reset token: %w", err)
	}

	return RenderPage(c, PageMeta{Title: "Choose a new password"}, views.PasswordResetConfirmPage(token))
}

// PasswordResetConfirm consumes the token and sets the new password.
func PasswordResetConfirm(c *echo.Context) error {
	if err := ParseForm(c); err != nil {
		return err
	}

	token := c.FormValue("token")
	password := c.FormValue("password")
	ctx := c.Request().Context()

	_, err := db.InTx(ctx, func(tx pgx.Tx) (auth.User, error) {
		return auth.ResetPasswordTx(ctx, tx, token, password)
	})
	if errors.Is(err, auth.ErrResetTokenInvalid) {
		SetFlash(c, "error", "that reset link is no longer valid — request a new one")
		return SafeRedirect(c, "/password-reset")
	}
	if msg := passwordPolicyMessage(err); msg != "" {
		SetFlash(c, "error", msg)
		return RenderPage(c, PageMeta{Title: "Choose a new password"}, views.PasswordResetConfirmPage(token))
	}
	if err != nil {
		return fmt.Errorf("resetting password: %w", err)
	}

	SetFlash(c, "ok", "password updated — sign in with your new password")
	return SafeRedirect(c, "/signin")
}

// passwordPolicyMessage maps auth's password-policy sentinels to flash
// text, or "" when err is something else.
func passwordPolicyMessage(err error) string {
	switch {
	case errors.Is(err, auth.ErrPasswordRequired):
		return "password is required"
	case errors.Is(err, auth.ErrPasswordTooShort):
		return fmt.Sprintf("password must be at least %d characters", auth.PasswordMinLength)
	case errors.Is(err, auth.ErrPasswordTooLong):
		return "that password is too long"
	}

	return ""
}

func Signout(c *echo.Context) error {
	if err := Sessions.Destroy(c.Request().Context()); err != nil {
		return fmt.Errorf("destroying session: %w", err)
	}

	return SafeRedirect(c, "/signin")
}

func Home(c *echo.Context) error {
	return RenderPage(c, PageMeta{Title: "Home"}, views.HomePage(CurrentUser(c).Name))
}
