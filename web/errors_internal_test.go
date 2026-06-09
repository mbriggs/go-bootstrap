package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

var errKaboom = errors.New("kaboom: connection refused")

// Serial — toggles the package-level dev flag. Internal so the toggle is a
// plain variable, not process-wide reconfiguration.
func TestErrorHandlerShowsDetailWithCopyButtonInDev(t *testing.T) {
	devMode = true
	t.Cleanup(func() { devMode = false })

	e := echo.New()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/boom", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	errorHandler(errKaboom, e.NewContext(req, rec))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "kaboom: connection refused") || !strings.Contains(body, "Copy error") {
		t.Fatalf("dev error page missing detail + copy button: %q", body)
	}
}

func TestErrorHandlerHidesDetailOutsideDev(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/boom", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	errorHandler(errKaboom, e.NewContext(req, rec))

	if strings.Contains(rec.Body.String(), "kaboom") {
		t.Fatal("internal detail leaked outside dev mode")
	}
}

// Serial — toggles the package-level production flag. The prod branch
// returns before any session use, so a bare context suffices.
func TestDesignShowcaseHiddenInProduction(t *testing.T) {
	prodMode = true
	t.Cleanup(func() { prodMode = false })

	e := echo.New()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/design", nil)
	rec := httptest.NewRecorder()

	err := DesignShowcase(e.NewContext(req, rec))
	if !errors.Is(err, echo.ErrNotFound) {
		t.Fatalf("err = %v, want echo.ErrNotFound", err)
	}
}
