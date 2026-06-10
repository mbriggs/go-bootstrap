// Smoke tests proving the harness and data layer work in a freshly
// bootstrapped project: they need no application tables, only a migrated
// (possibly empty) template database.
package db_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/webtest"
)

func TestMain(m *testing.M) {
	webtest.Main(m)
}

type row struct {
	ID int64 `db:"id"`
}

var errRollback = errors.New("rollback")

func TestFindOneTakesFirstRowOrNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	got, err := db.FindOne[row](ctx,
		"SELECT id FROM (VALUES (1::bigint), (2::bigint)) AS v(id) ORDER BY id")
	if err != nil {
		t.Fatalf("FindOne: %v", err)
	}
	if got.ID != 1 {
		t.Fatalf("got %+v, want the first row (ID 1)", got)
	}

	_, err = db.FindOne[row](ctx, "SELECT 1::bigint AS id WHERE false")
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("err = %v, want db.ErrNotFound", err)
	}
}

func TestFindExactlyOneRejectsAmbiguousMatches(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	got, err := db.FindExactlyOne[row](ctx, "SELECT 1::bigint AS id")
	if err != nil {
		t.Fatalf("FindExactlyOne: %v", err)
	}
	if got.ID != 1 {
		t.Fatalf("got %+v, want ID 1", got)
	}

	_, err = db.FindExactlyOne[row](ctx,
		"SELECT id FROM (VALUES (1::bigint), (2::bigint)) AS v(id)")
	if !errors.Is(err, db.ErrTooManyRows) {
		t.Fatalf("err = %v, want db.ErrTooManyRows", err)
	}

	_, err = db.FindExactlyOne[row](ctx, "SELECT 1::bigint AS id WHERE false")
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("err = %v, want db.ErrNotFound", err)
	}
}

func TestFindExactlyOneTxSeesUncommittedWrites(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, err := db.InTx(ctx, func(tx pgx.Tx) (row, error) {
		_, err := tx.Exec(ctx, "CREATE TEMP TABLE smoke (id bigint)")
		if err != nil {
			return row{}, fmt.Errorf("creating temp table: %w", err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO smoke (id) VALUES (42)"); err != nil {
			return row{}, fmt.Errorf("inserting row: %w", err)
		}

		got, err := db.FindExactlyOneTx[row](ctx, tx, "SELECT id FROM smoke")
		if err != nil {
			t.Errorf("FindExactlyOneTx could not see the transaction's own insert: %v", err)
		}

		if got.ID != 42 {
			t.Errorf("got %+v, want ID 42", got)
		}

		return got, errRollback
	})

	if !errors.Is(err, errRollback) {
		t.Fatalf("expected rollback error, got: %v", err)
	}
}
