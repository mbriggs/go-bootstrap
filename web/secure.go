package web

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v5"
)

// SecureHeaders sets the browser-side defense headers on every response.
// Scripts run only from our origin or with this request's CSP nonce —
// templ.WithNonce carries it to the layout's inline script and gesso's
// importmap. style-src allows inline because gesso components set style
// attributes (progress width, max-height); a style attribute can't run
// script, so that's an accepted trade. HSTS only in production — on
// localhost it would pin the browser to https the dev server doesn't speak.
func SecureHeaders(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		nonce := make([]byte, 16)
		if _, err := rand.Read(nonce); err != nil {
			return fmt.Errorf("generating csp nonce: %w", err)
		}
		n := base64.RawStdEncoding.EncodeToString(nonce)

		h := c.Response().Header()
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'nonce-"+n+"'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"form-action 'self'; "+
				"frame-ancestors 'none'; "+
				"base-uri 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if prodMode {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.SetRequest(c.Request().WithContext(templ.WithNonce(c.Request().Context(), n)))

		return next(c)
	}
}

// SameOriginPost rejects cross-origin POSTs (CSRF defense). Browsers send
// Origin on POST; older or stripped requests fall back to Sec-Fetch-Site.
// Non-browser clients must send one of the two.
func SameOriginPost(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
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
	// The Inngest endpoint authenticates by request signature, not session
	// cookie — CSRF needs an ambient cookie to ride, so there is nothing
	// here for the same-origin gate to protect, and the Inngest server is
	// legitimately cross-origin.
	if strings.HasPrefix(r.URL.Path, "/api/inngest") {
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
func SafeRedirect(c *echo.Context, path string) error {
	if !safeRedirectPath(path) {
		path = "/"
	}

	if err := c.Redirect(http.StatusSeeOther, path); err != nil {
		return fmt.Errorf("redirecting to %s: %w", path, err)
	}

	return nil
}
