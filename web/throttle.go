package web

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mbriggs/go-bootstrap/db"
)

// Signin throttling: bcrypt's cost slows offline cracking; this slows
// online guessing. Keys are (client IP, email) so one address can't walk
// the user list and one user can't be locked out from everywhere. Attempts
// live in Postgres (throttle_attempts) so the limit holds across processes
// — an in-memory count would silently stop throttling at the first
// horizontal scale-out.
const (
	throttleLimit  = 5
	throttleWindow = 15 * time.Minute
)

// ThrottleBlockedTx reports whether key has reached the attempt limit
// within the window.
func ThrottleBlockedTx(ctx context.Context, tx db.Queryable, key string) (bool, error) {
	rows, err := tx.Query(ctx,
		"SELECT count(*) FROM throttle_attempts WHERE key = $1 AND attempted_at > now() - make_interval(secs => $2)",
		key, throttleWindow.Seconds())
	if err != nil {
		return false, fmt.Errorf("counting throttle attempts: %w", err)
	}

	count, err := pgx.CollectExactlyOneRow(rows, pgx.RowTo[int64])
	if err != nil {
		return false, fmt.Errorf("collecting throttle count: %w", err)
	}

	return count >= throttleLimit, nil
}

// ThrottleFailTx records one attempt against key. Entries older than the
// window prune on the way in — attempts are rare, so the sweep rides along
// instead of needing a scheduled job.
func ThrottleFailTx(ctx context.Context, tx db.Queryable, key string) error {
	if _, err := tx.Exec(ctx,
		"DELETE FROM throttle_attempts WHERE attempted_at < now() - make_interval(secs => $1)",
		throttleWindow.Seconds()); err != nil {
		return fmt.Errorf("pruning throttle attempts: %w", err)
	}

	if _, err := tx.Exec(ctx,
		"INSERT INTO throttle_attempts (key) VALUES ($1)", key); err != nil {
		return fmt.Errorf("recording throttle attempt: %w", err)
	}

	return nil
}

// ThrottleResetTx clears key after a successful signin.
func ThrottleResetTx(ctx context.Context, tx db.Queryable, key string) error {
	if _, err := tx.Exec(ctx,
		"DELETE FROM throttle_attempts WHERE key = $1", key); err != nil {
		return fmt.Errorf("clearing throttle attempts: %w", err)
	}

	return nil
}
