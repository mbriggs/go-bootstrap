package web

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracing opens the request span. otelecho has no echo/v5 build (checked
// June 2026: not released, not on contrib main), so the span is opened by
// hand against the OTel API: propagation extract, a server span named for
// the route, response status from echo.ResolveResponseStatus. Like
// otelecho it records nothing until telemetry.Configure installs a
// provider. Retire this for the contrib middleware when it ships.
func Tracing(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		req := c.Request()

		// Provider and propagator resolve per request, so one installed
		// after the router was built (or swapped by a test) is honored.
		ctx := otel.GetTextMapPropagator().Extract(req.Context(), propagation.HeaderCarrier(req.Header))
		ctx, span := otel.GetTracerProvider().Tracer("github.com/mbriggs/go-bootstrap/web").Start(
			ctx, req.Method+" "+c.Path(),
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(req.Method),
				semconv.HTTPRoute(c.Path()),
				semconv.URLPath(req.URL.Path),
				semconv.ClientAddress(c.RealIP()),
			),
		)
		defer span.End()

		c.SetRequest(req.WithContext(ctx))

		err := next(c)

		// The error handler runs after the chain unwinds, so the response
		// isn't written yet on the error path; ResolveResponseStatus knows
		// what status it will produce.
		_, status := echo.ResolveResponseStatus(c.Response(), err)
		span.SetAttributes(semconv.HTTPResponseStatusCode(status))

		if err != nil {
			span.RecordError(err)
		}
		if status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(status))
		}

		return err
	}
}
