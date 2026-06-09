package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/mbriggs/pgsql"
)

var (
	ErrIncorrectMutation = errors.New("incorrect mutation")
	ErrMutationScope     = errors.New("incorrect number of rows mutated")
)

func UpdateTx(ctx context.Context, tx Queryable, update *pgsql.UpdateStatement) (int64, error) {
	sql, args := pgsql.Build(update)

	cmd, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return -1, err
	}

	if !cmd.Update() {
		return -1, fmt.Errorf("did not perform an update (sql: %s): %w", sql, ErrIncorrectMutation)
	}

	return cmd.RowsAffected(), nil
}

// InsertTx executes insert and scans its RETURNING row into T. The
// statement must include Returning(...) — without it Postgres returns no
// row and this reports ErrNotFound.
func InsertTx[T any](ctx context.Context, tx Queryable, insert *pgsql.InsertStatement) (T, error) {
	sql, args := pgsql.Build(insert)

	return FindTx[T](ctx, tx, sql, args...)
}

func DeleteTx(ctx context.Context, tx Queryable, del *pgsql.DeleteStatement) (int64, error) {
	sql, args := pgsql.Build(del)

	cmd, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return -1, err
	}

	if !cmd.Delete() {
		return -1, fmt.Errorf("did not perform a delete (sql: %s): %w", sql, ErrIncorrectMutation)
	}

	return cmd.RowsAffected(), nil
}
