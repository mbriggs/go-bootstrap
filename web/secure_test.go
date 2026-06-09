package web_test

import (
	"net/http"
	"net/url"
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
