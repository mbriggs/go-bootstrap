package web

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
)

// ParseForm parses the request form, converting a malformed body into a
// 400 instead of silently reading empty values. Call it at the top of any
// handler that reads form fields.
func ParseForm(c *echo.Context) error {
	if err := c.Request().ParseForm(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "malformed form").Wrap(err)
	}

	return nil
}

// UserMessager marks errors whose text is safe to show end users. Domain
// packages implement it on typed errors when the message should surface;
// everything else renders the caller's fallback.
type UserMessager interface{ UserMessage() string }

// UserMessage returns err's user-facing text, or fallback for internal
// errors.
func UserMessage(err error, fallback string) string {
	var um UserMessager
	if errors.As(err, &um) {
		return um.UserMessage()
	}

	return fallback
}

// FormResult describes the outcome of a POST mutation for FinishMutation.
type FormResult struct {
	Err           error
	ErrorFlashKey string // optional field key for the error flash
	OKMessage     string
	RedirectTo    string
}

// FinishMutation implements POST-redirect-GET: on error it logs the
// internal detail, flashes a safe message (keyed to a field when
// ErrorFlashKey is set), and redirects back; on success it flashes
// OKMessage and redirects on.
//
// Errors here never become 5xx — the redirect hides them from the error
// handler — so unexpected ones report to Sentry from this seam. A
// UserMessager is expected user feedback, not an incident.
func FinishMutation(c *echo.Context, result FormResult) error {
	if result.Err != nil {
		logger.Error(
			"mutation failed",
			"path", c.Request().URL.Path,
			"error", result.Err,
		)

		var um UserMessager
		if !errors.As(result.Err, &um) {
			captureError(result.Err, c)
		}

		SetKeyedFlash(c, "error", result.ErrorFlashKey, UserMessage(result.Err, "could not save changes"))
		return SafeRedirect(c, result.RedirectTo)
	}

	SetFlash(c, "ok", result.OKMessage)
	return SafeRedirect(c, result.RedirectTo)
}
