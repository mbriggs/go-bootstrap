package logging

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// traceHandler stamps trace_id and span_id onto records whose context
// carries an OTel span, so any log written with a *Context method
// (logger.InfoContext(ctx, ...)) correlates with its trace. Logs written
// without a context pass through untouched — there is nothing to
// correlate. Manager wraps every named logger's handler with this.
type traceHandler struct {
	slog.Handler
}

func (h traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}

	if err := h.Handler.Handle(ctx, r); err != nil {
		return fmt.Errorf("handling log record: %w", err)
	}

	return nil
}

// WithAttrs and WithGroup re-wrap so derived loggers (logger.With(...))
// keep stamping; embedding alone would return the bare inner handler.
func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{h.Handler.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{h.Handler.WithGroup(name)}
}
