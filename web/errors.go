package web

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v5"

	"github.com/mbriggs/go-bootstrap/logging"
	"github.com/mbriggs/go-bootstrap/views"
	"github.com/mbriggs/go-bootstrap/web/apierror"
)

var logger = logging.Logger("web")

// errorHandler renders failures: templ error pages for browsers, apierror
// JSON for API clients. Internal detail stays out of responses except in
// development, where the page shows it with a copy button.
func errorHandler(c *echo.Context, err error) {
	if resp, _ := echo.UnwrapResponse(c.Response()); resp != nil && resp.Committed {
		return
	}

	status := echo.StatusCode(err)
	if status == 0 {
		status = http.StatusInternalServerError
	}

	if status >= http.StatusInternalServerError {
		logger.Error(
			"request failed",
			"method", c.Request().Method,
			"uri", c.Request().RequestURI,
			"error", err,
		)
		captureError(err, c)
	}

	if !wantsHTML(c.Request()) {
		_ = apierror.JSON(c, status, http.StatusText(status))
		return
	}

	detail := ""
	if devMode {
		detail = err.Error()
	}

	data := views.LayoutData{
		Title: http.StatusText(status),
		User:  CurrentUser(c),
	}
	page := views.ErrorPage(views.ErrorPageData{
		Status:     status,
		StatusText: http.StatusText(status),
		Detail:     detail,
	})

	var buf bytes.Buffer
	if renderErr := views.Layout(data, page).Render(c.Request().Context(), &buf); renderErr != nil {
		// The error page itself failed — fall back to plain text rather
		// than recurse.
		http.Error(c.Response(), http.StatusText(status), status)
		return
	}

	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(status)
	_, _ = c.Response().Write(buf.Bytes())
}

func wantsHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// captureError reports a 5xx to Sentry — a no-op until
// telemetry.ConfigureSentry has installed a client (webtest never does).
// The hub is cloned per capture because the global hub's scope stack is
// not safe for concurrent requests. Only the user id goes on the event;
// SetRequest already scrubs cookies and auth headers.
func captureError(err error, c *echo.Context) {
	hub := sentry.CurrentHub().Clone()
	hub.Scope().SetRequest(c.Request())
	hub.Scope().SetTag("request_id", c.Response().Header().Get(echo.HeaderXRequestID))

	if u := CurrentUser(c); u != nil {
		hub.Scope().SetUser(sentry.User{ID: strconv.FormatInt(u.ID, 10)})
	}

	hub.CaptureException(err)
}
