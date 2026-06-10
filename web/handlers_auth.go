package web

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"

	"github.com/mbriggs/go-bootstrap/auth"
	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/jobs"
	"github.com/mbriggs/go-bootstrap/views"
)

func SigninForm(c echo.Context) error {
	if CurrentUser(c) != nil {
		return SafeRedirect(c, "/")
	}

	return RenderPage(c, PageMeta{Title: "Sign in"}, views.SigninPage(""))
}

func SigninSubmit(c echo.Context) error {
	if err := ParseForm(c); err != nil {
		return err
	}

	email := strings.TrimSpace(c.FormValue("email"))
	password := c.FormValue("password")
	ctx := c.Request().Context()

	throttleKey := c.RealIP() + "|" + strings.ToLower(email)
	if SigninThrottle.Blocked(throttleKey) {
		SetFlash(c, "error", "too many failed attempts — try again later")
		return RenderPage(c, PageMeta{Title: "Sign in"}, views.SigninPage(email))
	}

	user, err := auth.Authenticate(ctx, email, password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		SigninThrottle.Fail(throttleKey)
		SetFlash(c, "error", "invalid email or password")
		return RenderPage(c, PageMeta{Title: "Sign in"}, views.SigninPage(email))
	}
	if err != nil {
		return fmt.Errorf("authenticating: %w", err)
	}
	SigninThrottle.Reset(throttleKey)

	// Rotate the session id on signin to defend against fixation.
	if err := Sessions.RenewToken(ctx); err != nil {
		return fmt.Errorf("renewing session: %w", err)
	}
	Sessions.Put(ctx, "user_id", user.ID)

	return SafeRedirect(c, Sessions.PopString(ctx, "after_signin"))
}

// PasswordResetRequestForm renders the "enter your email" form.
func PasswordResetRequestForm(c echo.Context) error {
	return RenderPage(c, PageMeta{Title: "Reset password"}, views.PasswordResetRequestPage(""))
}

// PasswordResetRequest enqueues the reset email. The response never
// reveals whether the email exists, and every request counts against the
// throttle — this endpoint sends mail, so it throttles on attempts, not
// failures.
func PasswordResetRequest(c echo.Context) error {
	if err := ParseForm(c); err != nil {
		return err
	}

	email := strings.TrimSpace(c.FormValue("email"))
	ctx := c.Request().Context()

	throttleKey := "reset|" + c.RealIP() + "|" + strings.ToLower(email)
	if SigninThrottle.Blocked(throttleKey) {
		SetFlash(c, "error", "too many reset requests — try again later")
		return RenderPage(c, PageMeta{Title: "Reset password"}, views.PasswordResetRequestPage(email))
	}
	SigninThrottle.Fail(throttleKey)

	// No InsertTx here because there is no accompanying state change — the
	// worker mints the token at send time. Unknown email enqueues nothing
	// and answers identically.
	user, err := auth.ByEmail(ctx, email)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return fmt.Errorf("looking up reset account: %w", err)
	}
	if err == nil {
		if _, err := jobs.Client.Insert(ctx, jobs.PasswordResetEmailArgs{Email: user.Email}, nil); err != nil {
			return fmt.Errorf("enqueueing reset email: %w", err)
		}
	}

	SetFlash(c, "ok", "if that email has an account, a reset link is on the way")
	return SafeRedirect(c, "/signin")
}

// PasswordResetConfirmForm checks the emailed token before showing the
// new-password form, so a dead link fails before the user types anything.
func PasswordResetConfirmForm(c echo.Context) error {
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
func PasswordResetConfirm(c echo.Context) error {
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
	if errors.Is(err, auth.ErrPasswordRequired) {
		SetFlash(c, "error", "password is required")
		return RenderPage(c, PageMeta{Title: "Choose a new password"}, views.PasswordResetConfirmPage(token))
	}
	if err != nil {
		return fmt.Errorf("resetting password: %w", err)
	}

	SetFlash(c, "ok", "password updated — sign in with your new password")
	return SafeRedirect(c, "/signin")
}

func Signout(c echo.Context) error {
	if err := Sessions.Destroy(c.Request().Context()); err != nil {
		return fmt.Errorf("destroying session: %w", err)
	}

	return SafeRedirect(c, "/signin")
}

func Home(c echo.Context) error {
	return RenderPage(c, PageMeta{Title: "Home"}, views.HomePage(CurrentUser(c).Name))
}
