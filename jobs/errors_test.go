package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/riverqueue/river/rivertype"
)

var errSMTPDown = errors.New("smtp down")

func TestJobFailuresReportToSentryOnlyWhenDiscarded(t *testing.T) {
	events := captureSentryEvents(t)
	h := sentryErrorHandler{}

	job := &rivertype.JobRow{ID: 7, Kind: "password_reset_email", Attempt: 1, MaxAttempts: 3}
	h.HandleError(context.Background(), job, errSMTPDown)
	if len(*events) != 0 {
		t.Fatalf("captured %d events for a retryable failure, want 0", len(*events))
	}

	job.Attempt = 3
	h.HandleError(context.Background(), job, errSMTPDown)
	if len(*events) != 1 {
		t.Fatalf("captured %d events for a discarded job, want 1", len(*events))
	}
	if (*events)[0].Tags["job"] != "password_reset_email" {
		t.Fatalf("event tags = %+v, want job kind", (*events)[0].Tags)
	}
}

func TestJobPanicsAlwaysReportToSentry(t *testing.T) {
	events := captureSentryEvents(t)
	h := sentryErrorHandler{}

	job := &rivertype.JobRow{ID: 7, Kind: "password_reset_email", Attempt: 1, MaxAttempts: 3}
	h.HandlePanic(context.Background(), job, "nil pointer", "goroutine 1 [running]: ...")
	if len(*events) != 1 {
		t.Fatalf("captured %d events for a first-attempt panic, want 1", len(*events))
	}
}

// captureSentryEvents installs a test client whose BeforeSend records
// events and then drops them, so nothing leaves the process.
func captureSentryEvents(t *testing.T) *[]*sentry.Event {
	t.Helper()

	var events []*sentry.Event
	err := sentry.Init(sentry.ClientOptions{
		Dsn: "https://key@sentry.invalid/1",
		BeforeSend: func(ev *sentry.Event, _ *sentry.EventHint) *sentry.Event {
			events = append(events, ev)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("sentry.Init: %v", err)
	}
	t.Cleanup(func() { sentry.CurrentHub().BindClient(nil) })

	return &events
}
