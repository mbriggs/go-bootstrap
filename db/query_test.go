// Smoke tests proving the harness and data layer work in a freshly
// bootstrapped project: they need no application tables, only a migrated
// (possibly empty) template database.
package db_test

import (
	"context"
	"errors"
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

func TestFind(t *testing.T) {
	t.Parallel()

	got, err := db.Find[row](context.Background(), "SELECT 1::bigint AS id")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if got.ID != 1 {
		t.Fatalf("got %+v, want ID 1", got)
	}
}

func TestFindTxSeesUncommittedWrites(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, err := db.InTx(ctx, func(tx pgx.Tx) (row, error) {
		_, err := tx.Exec(ctx, "CREATE TEMP TABLE smoke (id bigint)")
		if err != nil {
			return row{}, err
		}

		if _, err := tx.Exec(ctx, "INSERT INTO smoke (id) VALUES (42)"); err != nil {
			return row{}, err
		}

		got, err := db.FindTx[row](ctx, tx, "SELECT id FROM smoke")
		if err != nil {
			t.Errorf("FindTx could not see the transaction's own insert: %v", err)
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
