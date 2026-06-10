package web_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"

	"github.com/mbriggs/go-bootstrap/webtest"
)

var errKaboom = errors.New("kaboom")

func TestNotFoundRendersHTMLErrorPageForBrowsers(t *testing.T) {
	client := webtest.NewClient(t, webtest.Server(t.Context()))

	rec := client.Do(http.MethodGet, "/nope", nil, map[string]string{"Accept": "text/html"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "404 Not Found") {
		t.Fatalf("body missing 404 heading: %q", body)
	}
	if strings.Contains(body, "Copy error") {
		t.Fatal("test env should not render the dev error detail")
	}
}

func TestServerErrorsReportToSentry(t *testing.T) {
	events := captureSentryEvents(t)

	e := webtest.Server(t.Context())
	e.GET("/boom", func(echo.Context) error { return errKaboom })
	client := webtest.NewClient(t, e)

	rec := client.Do(http.MethodGet, "/boom", nil, map[string]string{"Accept": "text/html"})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	if len(*events) != 1 {
		t.Fatalf("captured %d sentry events, want 1", len(*events))
	}
	ev := (*events)[0]
	if len(ev.Exception) == 0 || !strings.Contains(ev.Exception[0].Value, "kaboom") {
		t.Fatalf("event exception = %+v, want the handler error", ev.Exception)
	}
	if ev.Tags["request_id"] == "" {
		t.Fatal("event missing request_id tag")
	}
	if ev.Request == nil || !strings.Contains(ev.Request.URL, "/boom") {
		t.Fatalf("event request = %+v, want the failing request attached", ev.Request)
	}

	// Non-5xx responses are expected failures, not incidents.
	client.Do(http.MethodGet, "/nope", nil, map[string]string{"Accept": "text/html"})
	if len(*events) != 1 {
		t.Fatalf("captured %d sentry events after a 404, want still 1", len(*events))
	}
}

// captureSentryEvents installs a test client whose BeforeSend records
// events and then drops them, so nothing leaves the process.
func captureSentryEvents(t *testing.T) *[]*sentry.Event {
	t.Helper()

	var events []*sentry.Event
	err := sentry.Init(sentry.ClientOptions{
		Dsn: "https://key@sentry.invalid/1",
		BeforeSend: func(ev *sentry.Event, _ *sentry.EventHint) *sentry.Event {
			events = append(events, ev)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("sentry.Init: %v", err)
	}
	t.Cleanup(func() { sentry.CurrentHub().BindClient(nil) })

	return &events
}

func TestNotFoundRendersJSONForAPIClients(t *testing.T) {
	client := webtest.NewClient(t, webtest.Server(t.Context()))

	rec := client.Do(http.MethodGet, "/nope", nil, map[string]string{"Accept": "application/json"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}

	var resp struct {
		Message string `json:"message"`
		Status  int    `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (%q)", err, rec.Body.String())
	}
	if resp.Status != http.StatusNotFound || resp.Message == "" {
		t.Fatalf("resp = %+v, want apierror shape", resp)
	}
}
