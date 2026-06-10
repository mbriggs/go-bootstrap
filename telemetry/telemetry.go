// Package telemetry wires OpenTelemetry tracing, gated on the standard SDK
// environment variables (the OTEL_* family is SDK-consumed configuration,
// like PG* — it doesn't go through env.Load). With no
// OTEL_EXPORTER_OTLP_ENDPOINT set, Configure leaves the global no-op
// provider in place: the echo middleware and pgx tracer stay installed and
// record nothing, which costs effectively nothing. Set the endpoint and
// every request and query ships as OTLP spans.
//
//	OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318  # enables tracing
//	OTEL_SERVICE_NAME=myapp                            # optional; defaults to the module name
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/mbriggs/go-bootstrap/appname"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

// Configure installs the OTLP trace provider when an endpoint is
// configured. The returned shutdown flushes pending spans; main defers it.
func Configure(ctx context.Context) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" || os.Getenv("OTEL_SDK_DISABLED") == "true" {
		return noop, nil
	}

	// Endpoint, headers, TLS, and timeouts all come from the
	// OTEL_EXPORTER_OTLP_* environment variables the exporter reads itself.
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return noop, fmt.Errorf("creating otlp exporter: %w", err)
	}

	// resource.Default honors OTEL_SERVICE_NAME / OTEL_RESOURCE_ATTRIBUTES;
	// the merge supplies the module-derived fallback service name.
	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(ServiceName())))
	if err != nil {
		return noop, fmt.Errorf("merging resource attributes: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return func(ctx context.Context) error {
		return errors.Join(tp.ForceFlush(ctx), tp.Shutdown(ctx))
	}, nil
}

// ServiceName is OTEL_SERVICE_NAME when set, otherwise the module's base
// name — the same derive-from-module convention as database names.
func ServiceName() string {
	if name := os.Getenv("OTEL_SERVICE_NAME"); name != "" {
		return name
	}

	if name := appname.Base(); name != "" {
		return name
	}

	return "app"
}
