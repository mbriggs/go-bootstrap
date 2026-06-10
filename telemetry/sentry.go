package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/getsentry/sentry-go"
)

var errSentryFlushTimeout = errors.New("sentry flush timed out; some events were dropped")

// ConfigureSentry installs the Sentry client when SENTRY_DSN is set —
// the SENTRY_* family is SDK-consumed configuration like OTEL_*, so it
// doesn't go through env.Load. With no DSN, every capture site stays a
// no-op against the empty global hub. The returned flush drains pending
// events; main defers it.
//
// environment is APP_ENV — the app's one source of environment truth, so
// SENTRY_ENVIRONMENT is deliberately not a second knob. SENTRY_RELEASE
// tags events with a deploy version when set.
func ConfigureSentry(environment string) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }

	if os.Getenv("SENTRY_DSN") == "" {
		return noop, nil
	}

	err := sentry.Init(sentry.ClientOptions{
		Environment: environment,
	})
	if err != nil {
		return noop, fmt.Errorf("initializing sentry: %w", err)
	}

	return func(ctx context.Context) error {
		if !sentry.FlushWithContext(ctx) {
			return errSentryFlushTimeout
		}

		return nil
	}, nil
}
