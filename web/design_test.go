package web_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mbriggs/go-bootstrap/webtest"
)

func TestDesignShowcaseRendersGalleryOutsideProduction(t *testing.T) {
	client := webtest.NewClient(t, webtest.Server(t.Context()))

	rec := client.Do(http.MethodGet, "/design", nil, map[string]string{"Accept": "text/html"})
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /design = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Design system", "design-section", "data-ui-scrollspy-section"} {
		if !strings.Contains(body, want) {
			t.Fatalf("design page missing %q", want)
		}
	}
}

func TestUIAssetsAreServed(t *testing.T) {
	client := webtest.NewClient(t, webtest.Server(t.Context()))

	rec := client.Get("/ui/ui.css")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "@layer") {
		t.Fatalf("GET /ui/ui.css = %d, want the stylesheet entry", rec.Code)
	}
}
