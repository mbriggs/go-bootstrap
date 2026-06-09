package web

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/mbriggs/go-bootstrap/logging"
	"github.com/mbriggs/go-bootstrap/views"
	"github.com/mbriggs/go-bootstrap/web/apierror"
)

var logger = logging.Logger("web")

// errorHandler renders failures: templ error pages for browsers, apierror
// JSON for API clients. Internal detail stays out of responses except in
// development, where the page shows it with a copy button.
func errorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	status := http.StatusInternalServerError
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		status = httpErr.Code
	}

	if status >= http.StatusInternalServerError {
		logger.Error(
			"request failed",
			"method", c.Request().Method,
			"uri", c.Request().RequestURI,
			"error", err,
		)
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
