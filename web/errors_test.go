package web_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mbriggs/go-bootstrap/webtest"
)

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
