package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/mbriggs/go-bootstrap/webtest"
)

// Behind a TLS-terminating proxy the browser sends an https Origin while
// the local connection is plain HTTP — X-Forwarded-Proto must reconcile
// the two or every same-origin POST 403s on deploy.
func TestSameOriginPostAcceptsHTTPSOriginBehindProxy(t *testing.T) {
	client := webtest.NewClient(t, webtest.Server(t.Context()))

	rec := client.Do(http.MethodPost, "/signin",
		url.Values{"email": {"proxy@example.com"}, "password": {"pw"}},
		map[string]string{
			"Sec-Fetch-Site":    "",
			"Origin":            "https://example.com",
			"X-Forwarded-Proto": "https",
		})
	// Same-origin gate passes; bad credentials re-render the form (200),
	// which is all this test cares about — not 403.
	if rec.Code == http.StatusForbidden {
		t.Fatalf("status = 403; the same-origin gate rejected a proxied https request")
	}
}

func TestSecureHeadersOnEveryResponse(t *testing.T) {
	client := webtest.NewClient(t, webtest.Server(t.Context()))
	rec := client.Get("/signin")

	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	// HSTS is production-only — pinning localhost to https would break the
	// dev loop.
	if hsts := rec.Header().Get("Strict-Transport-Security"); hsts != "" {
		t.Errorf("Strict-Transport-Security = %q, want unset outside production", hsts)
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("CSP missing baseline directives: %q", csp)
	}

	// The header's nonce must be the one the page's inline scripts carry,
	// or the browser blocks them.
	m := regexp.MustCompile(`'nonce-([^']+)'`).FindStringSubmatch(csp)
	if m == nil {
		t.Fatalf("CSP has no script nonce: %q", csp)
	}
	if !strings.Contains(rec.Body.String(), `nonce="`+m[1]+`"`) {
		t.Fatal("page body carries no script tag with the CSP header's nonce")
	}
}

// Tripwire for the gesso pin: the importmap is an inline script gesso
// renders, so it must carry the CSP nonce or browsers block it and every
// page silently loses its JS — a failure no other test can see. This fails
// against a gesso version that predates nonce support (< v0.2.0), which is
// exactly the point: the pin can't lag the CSP.
func TestImportMapCarriesCSPNonce(t *testing.T) {
	client := webtest.NewClient(t, webtest.Server(t.Context()))
	rec := client.Get("/signin")

	m := regexp.MustCompile(`'nonce-([^']+)'`).FindStringSubmatch(rec.Header().Get("Content-Security-Policy"))
	if m == nil {
		t.Fatal("CSP has no script nonce")
	}
	if !strings.Contains(rec.Body.String(), `<script type="importmap" nonce="`+m[1]+`">`) {
		t.Fatal("importmap script missing the CSP nonce — bump the gesso pin to a nonce-aware version")
	}
}

func TestSecureHeadersNonceIsPerRequest(t *testing.T) {
	client := webtest.NewClient(t, webtest.Server(t.Context()))

	first := client.Get("/signin").Header().Get("Content-Security-Policy")
	second := client.Get("/signin").Header().Get("Content-Security-Policy")
	if first == second {
		t.Fatal("CSP nonce repeated across requests; it must be fresh per response")
	}
}

func TestOversizedBodyIsRejected(t *testing.T) {
	client := webtest.NewClient(t, webtest.Server(t.Context()))

	rec := client.PostForm("/signin", url.Values{
		"email":    {"big@example.com"},
		"password": {strings.Repeat("a", 3<<20)},
	})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("3MiB POST = %d, want 413", rec.Code)
	}
}

func TestSameOriginPostRejectsMismatchedOrigin(t *testing.T) {
	client := webtest.NewClient(t, webtest.Server(t.Context()))

	rec := client.Do(http.MethodPost, "/signin",
		url.Values{"email": {"evil@example.com"}, "password": {"pw"}},
		map[string]string{
			"Sec-Fetch-Site": "",
			"Origin":         "https://evil.example.net",
		})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for cross-origin POST", rec.Code)
	}
}
