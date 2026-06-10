package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/getsentry/sentry-go"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

var errJobPanic = errors.New("job panic")

// sentryErrorHandler reports job failures to Sentry — a no-op until
// telemetry.ConfigureSentry has installed a client. River already logs and
// retries every failure, so errors report only on the final attempt, when
// the job is discarded and a human has to act. Panics report immediately:
// a panic is a bug, not a transient failure, and waiting out the retry
// schedule buries the signal.
type sentryErrorHandler struct{}

func (sentryErrorHandler) HandleError(_ context.Context, job *rivertype.JobRow, err error) *river.ErrorHandlerResult {
	if job.Attempt < job.MaxAttempts {
		return nil
	}

	captureJobFailure(job, err, "")

	return nil
}

func (sentryErrorHandler) HandlePanic(_ context.Context, job *rivertype.JobRow, panicVal any, trace string) *river.ErrorHandlerResult {
	captureJobFailure(job, fmt.Errorf("%w: %v", errJobPanic, panicVal), trace)

	return nil
}

func captureJobFailure(job *rivertype.JobRow, err error, trace string) {
	jobContext := map[string]any{
		"id":           job.ID,
		"kind":         job.Kind,
		"attempt":      job.Attempt,
		"max_attempts": job.MaxAttempts,
		"queue":        job.Queue,
	}
	if trace != "" {
		jobContext["panic_trace"] = trace
	}

	hub := sentry.CurrentHub().Clone()
	hub.Scope().SetTag("job", job.Kind)
	hub.Scope().SetContext("job", jobContext)
	hub.CaptureException(err)
}
