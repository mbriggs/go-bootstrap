package web

import (
	"errors"
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/mbriggs/go-bootstrap/auth"
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

func Signout(c echo.Context) error {
	if err := Sessions.Destroy(c.Request().Context()); err != nil {
		return fmt.Errorf("destroying session: %w", err)
	}

	return SafeRedirect(c, "/signin")
}

func Home(c echo.Context) error {
	return RenderPage(c, PageMeta{Title: "Home"}, views.HomePage(CurrentUser(c).Name))
}
