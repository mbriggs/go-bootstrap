package webtest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

type AppRequest struct {
	*PathState
	Method       string
	Body         any
	ResponseCode int
	Handler      echo.HandlerFunc
}

func Request[T any](t *testing.T, appReq AppRequest) T {
	t.Helper()

	logger.Debug(
		"Request",
		"method", appReq.Method,
		"path", appReq.Path(),
	)

	// Build Request
	var req *http.Request
	if appReq.Body == nil {
		req = httptest.NewRequestWithContext(t.Context(), appReq.Method, appReq.Path(), nil)
	} else {
		body, err := json.Marshal(appReq.Body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}

		req = httptest.NewRequestWithContext(t.Context(), appReq.Method, appReq.Path(), bytes.NewBuffer(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}

	// Build Context
	rec := httptest.NewRecorder()
	c := Echo.NewContext(req, rec)

	pn, pv := appReq.RoutedState()
	c.SetParamNames(pn...)
	c.SetParamValues(pv...)

	// Invoke Handler
	logger.Info("Requesting", "method", appReq.Method, "path", appReq.Path())

	err := appReq.Handler(c)
	if err != nil {
		t.Errorf("handler for %s failed: %v", appReq.Path(), err)
	}

	logger.Info("Request completed", "method", appReq.Method, "path", appReq.Path())

	if rec.Code != appReq.ResponseCode {
		t.Fatalf("Looking for status code %d, got %d, with response: \"%s\"", appReq.ResponseCode, rec.Code, rec.Body.String())
	}

	// Build Response
	var response T
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	return response
}
