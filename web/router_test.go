package web_test

import (
	"net/http"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mbriggs/go-bootstrap/webtest"
)

// Not parallel: swaps the global tracer provider.
func TestRequestIDRidesTheRequestSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	client := webtest.NewClient(t, webtest.Server(t.Context()))
	rec := client.Do(http.MethodGet, "/signin", nil, map[string]string{"Accept": "text/html"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	requestID := rec.Header().Get("X-Request-Id")
	if requestID == "" {
		t.Fatal("response missing X-Request-Id header")
	}

	for _, span := range recorder.Ended() {
		for _, attr := range span.Attributes() {
			if attr.Key == "http.request_id" && attr.Value.AsString() == requestID {
				return
			}
		}
	}

	t.Fatalf("no recorded span carries http.request_id=%q (%d spans recorded)", requestID, len(recorder.Ended()))
}
