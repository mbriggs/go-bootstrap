package db_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/mbriggs/go-bootstrap/db"
)

// Not parallel: hstore registration happens per connection at connect
// time, so after creating the extension the pool is rebuilt — serial
// tests run before any t.Parallel() test starts, so nothing else holds
// the old pool.
func TestHstoreScansIntoMap(t *testing.T) {
	ctx := context.Background()

	current, err := db.Find[dbName](ctx, "SELECT current_database() AS name")
	if err != nil {
		t.Fatalf("reading current database: %v", err)
	}

	err = db.ExecInTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS hstore"); err != nil {
			return fmt.Errorf("creating hstore extension: %w", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ExecInTx: %v", err)
	}

	db.Close()
	if err := db.Configure(ctx, "dbname="+current.Name); err != nil {
		t.Fatalf("rebuilding pool: %v", err)
	}

	type row struct {
		Tags map[string]string `db:"tags"`
	}

	got, err := db.Find[row](ctx, "SELECT 'a=>1,b=>2'::hstore AS tags")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if got.Tags["a"] != "1" || got.Tags["b"] != "2" {
		t.Fatalf("got %+v, want map with a=1 b=2", got.Tags)
	}
}

type dbName struct {
	Name string `db:"name"`
}
