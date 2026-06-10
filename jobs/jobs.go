// Package jobs is the background-job tier, backed by River. Jobs are
// transport: a worker's Work unpacks args and calls domain code — behavior
// stays in domain packages. Enqueue with Client.InsertTx inside the same
// transaction as the state change the job follows from; the job only
// becomes runnable when that transaction commits, so a rolled-back request
// can never leave an orphaned job (this transactional handoff is why River
// over an external broker — coupling to Postgres is a feature here).
// Enqueues with no accompanying state change go through InsertStandalone;
// the jobconfine analyzer keeps River's plain Insert confined to this
// package so that claim is always explicit.
//
// For multi-step orchestration with waits and event coordination, see the
// flows package — the dividing line lives there.
package jobs

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"github.com/mbriggs/go-bootstrap/logging"
)

var logger = logging.Logger("jobs")

// Client is the process-wide River client — like db.Conn, thread-safe and
// singular. Configure builds it; only the real server process calls Start
// (webtest configures without starting, so tests can assert on enqueued
// rows without jobs running underneath them).
var Client *river.Client[pgx.Tx]

// Configure builds the client and registers every worker. baseURL is the
// externally reachable origin used in emailed links.
func Configure(pool *pgxpool.Pool, baseURL string) error {
	workers := river.NewWorkers()
	river.AddWorker(workers, &passwordResetEmailWorker{baseURL: baseURL})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
		},
		Workers:      workers,
		Logger:       logger,
		ErrorHandler: sentryErrorHandler{},
	})
	if err != nil {
		return fmt.Errorf("creating river client: %w", err)
	}

	Client = client

	return nil
}

// InsertStandalone enqueues args outside any transaction. The name is the
// claim the call site makes: no state change accompanies this enqueue, so
// there is nothing for a rollback to orphan. When the enqueue does follow
// a state change, use Client.InsertTx in that transaction instead — the
// jobconfine analyzer confines plain Insert here so the choice is always
// explicit at the call site.
func InsertStandalone(ctx context.Context, args river.JobArgs) (*rivertype.JobInsertResult, error) {
	res, err := Client.Insert(ctx, args, nil)
	if err != nil {
		return nil, fmt.Errorf("enqueueing standalone job: %w", err)
	}

	return res, nil
}

// Start begins working jobs. Call once at boot, after Configure.
func Start(ctx context.Context) error {
	if err := Client.Start(ctx); err != nil {
		return fmt.Errorf("starting river client: %w", err)
	}

	return nil
}

// Stop drains in-flight jobs until ctx expires, then cancels whatever is
// still running.
func Stop(ctx context.Context) error {
	if err := Client.Stop(ctx); err != nil {
		logger.Error("soft stop expired, cancelling running jobs", "error", err)

		if err := Client.StopAndCancel(context.WithoutCancel(ctx)); err != nil {
			return fmt.Errorf("cancelling running jobs: %w", err)
		}
	}

	return nil
}
