package db_test

import (
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/mbriggs/pgsql"

	"github.com/mbriggs/go-bootstrap/db"
)

func TestInsertTxReturnsRowAndDeleteTxRemovesIt(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	type row struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}

	err := db.ExecInTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "CREATE TEMP TABLE mutate_smoke (id bigint GENERATED ALWAYS AS IDENTITY, name text NOT NULL)"); err != nil {
			return fmt.Errorf("creating temp table: %w", err)
		}

		inserted, err := db.InsertTx[row](ctx, tx,
			pgsql.Insert("mutate_smoke").Data(pgsql.RowMap{"name": "a"}).Returning("id, name"))
		if err != nil {
			return fmt.Errorf("insert: %w", err)
		}
		if inserted.ID == 0 || inserted.Name != "a" {
			t.Errorf("inserted = %+v, want generated id and name a", inserted)
		}

		n, err := db.DeleteTx(ctx, tx, pgsql.Delete("mutate_smoke").Where("id = ?", inserted.ID))
		if err != nil {
			return fmt.Errorf("delete: %w", err)
		}
		if n != 1 {
			t.Errorf("deleted %d rows, want 1", n)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
}
