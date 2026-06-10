package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestTraceHandlerStampsTraceAndSpanIDs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(traceHandler{slog.NewTextHandler(&buf, nil)})

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x01},
		SpanID:     trace.SpanID{0x02},
		TraceFlags: trace.FlagsSampled,
	})
	logger.InfoContext(trace.ContextWithSpanContext(context.Background(), sc), "hello")

	out := buf.String()
	if !strings.Contains(out, "trace_id=01000000000000000000000000000000") {
		t.Fatalf("output missing trace_id: %q", out)
	}
	if !strings.Contains(out, "span_id=0200000000000000") {
		t.Fatalf("output missing span_id: %q", out)
	}
}

func TestTraceHandlerLeavesUntracedLogsAlone(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(traceHandler{slog.NewTextHandler(&buf, nil)})

	logger.Info("hello")
	logger.InfoContext(context.Background(), "with ctx but no span")

	if strings.Contains(buf.String(), "trace_id") {
		t.Fatalf("untraced log got a trace_id: %q", buf.String())
	}
}

func TestTraceHandlerSurvivesWith(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(traceHandler{slog.NewTextHandler(&buf, nil)}).With("k", "v")

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x01},
		SpanID:     trace.SpanID{0x02},
		TraceFlags: trace.FlagsSampled,
	})
	logger.InfoContext(trace.ContextWithSpanContext(context.Background(), sc), "hello")

	if !strings.Contains(buf.String(), "trace_id=") {
		t.Fatalf("With() dropped the trace wrapper: %q", buf.String())
	}
}
