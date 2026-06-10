package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Queryable interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func FindAllTx[T any](ctx context.Context, tx Queryable, sql string, args ...any) ([]T, error) {
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("db search error querying: %w", err)
	}

	defer rows.Close()

	result, err := pgx.CollectRows(rows, pgx.RowToStructByName[T])
	if err != nil {
		return nil, fmt.Errorf("db search error collecting: %w", err)
	}

	return result, nil
}

// FindOneTx returns the first row scanned into T, or ErrNotFound when there
// are none. Extra rows are not an error — reach for this when the query
// orders and the first row is the answer (latest-by, top-ranked). For
// unique-predicate lookups use FindExactlyOneTx, which makes an ambiguous
// match loud instead of silently picking one.
func FindOneTx[T any](ctx context.Context, tx Queryable, sql string, args ...any) (T, error) {
	var result T

	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return result, fmt.Errorf("db find error querying: %w", err)
	}

	result, err = pgx.CollectOneRow(rows, pgx.RowToStructByName[T])
	if errors.Is(err, pgx.ErrNoRows) {
		return result, fmt.Errorf("db find error: %w", ErrNotFound)
	}
	if err != nil {
		return result, fmt.Errorf("db find error collecting: %w", err)
	}

	return result, nil
}

// FindExactlyOneTx returns the single row scanned into T: ErrNotFound when
// there are none, ErrTooManyRows when there is more than one. The default
// for unique-predicate lookups — a second matching row means the predicate
// isn't as unique as the caller believed, which is a bug to surface, not a
// row to pick.
func FindExactlyOneTx[T any](ctx context.Context, tx Queryable, sql string, args ...any) (T, error) {
	var result T

	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return result, fmt.Errorf("db find error querying: %w", err)
	}

	result, err = pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[T])
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return result, fmt.Errorf("db find error: %w", ErrNotFound)
	case errors.Is(err, pgx.ErrTooManyRows):
		return result, fmt.Errorf("db find error: %w", ErrTooManyRows)
	case err != nil:
		return result, fmt.Errorf("db find error collecting: %w", err)
	}

	return result, nil
}
