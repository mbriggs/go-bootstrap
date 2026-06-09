package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
)

// SameOriginPost rejects cross-origin POSTs (CSRF defense). Browsers send
// Origin on POST; older or stripped requests fall back to Sec-Fetch-Site.
// Non-browser clients must send one of the two.
func SameOriginPost(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !sameOriginPost(c.Request()) {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}

		return next(c)
	}
}

func sameOriginPost(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return true
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		return err == nil &&
			strings.EqualFold(u.Scheme, requestScheme(r)) &&
			strings.EqualFold(u.Host, r.Host)
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return true
	default:
		return false
	}
}

// requestScheme resolves the request's external scheme. Behind a
// TLS-terminating proxy r.TLS is nil while the browser's Origin says
// https, so X-Forwarded-Proto wins when present. Trusting it is safe for
// the same-origin check: a cross-site form post cannot attach custom
// headers, so an attacker can't use it to make the comparison pass.
func requestScheme(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if r.URL.Scheme != "" {
		return r.URL.Scheme
	}
	if r.TLS != nil {
		return "https"
	}

	return "http"
}

func safeRedirectPath(path string) bool {
	return strings.HasPrefix(path, "/") &&
		!strings.HasPrefix(path, "//") &&
		!strings.HasPrefix(path, `/\`)
}

// SafeRedirect issues a 303 See Other to an in-app path after enforcing
// that the destination is local (leading slash, no scheme/host, no
// protocol-relative escapes). All handlers should funnel POST→GET
// redirects through here so redirect validation stays in one place.
func SafeRedirect(c echo.Context, path string) error {
	if !safeRedirectPath(path) {
		path = "/"
	}

	if err := c.Redirect(http.StatusSeeOther, path); err != nil {
		return fmt.Errorf("redirecting to %s: %w", path, err)
	}

	return nil
}
