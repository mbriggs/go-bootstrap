package web_test

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/mbriggs/go-bootstrap/web"
	"github.com/mbriggs/go-bootstrap/webtest"
)

var errDBExploded = errors.New("db exploded")

type expectedError struct{}

func (expectedError) Error() string       { return "email taken" }
func (expectedError) UserMessage() string { return "that email is already in use" }

// Unexpected mutation failures never become 5xx — FinishMutation flashes
// and redirects — so they report to Sentry from there. Expected domain
// errors (UserMessager) are user feedback, not incidents.
func TestFinishMutationReportsOnlyUnexpectedErrorsToSentry(t *testing.T) {
	events := captureSentryEvents(t)

	e := webtest.Server(t.Context())
	e.POST("/surprise", func(c *echo.Context) error {
		return web.FinishMutation(c, web.FormResult{Err: errDBExploded, RedirectTo: "/signin"})
	})
	e.POST("/expected", func(c *echo.Context) error {
		return web.FinishMutation(c, web.FormResult{Err: expectedError{}, RedirectTo: "/signin"})
	})
	client := webtest.NewClient(t, e)

	rec := client.Do(http.MethodPost, "/surprise", url.Values{}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if len(*events) != 1 {
		t.Fatalf("captured %d events for an unexpected mutation error, want 1", len(*events))
	}
	if ev := (*events)[0]; len(ev.Exception) == 0 || !strings.Contains(ev.Exception[0].Value, "db exploded") {
		t.Fatalf("event exception = %+v, want the mutation error", ev.Exception)
	}

	client.Do(http.MethodPost, "/expected", url.Values{}, nil)
	if len(*events) != 1 {
		t.Fatalf("captured %d events after an expected domain error, want still 1", len(*events))
	}
}
