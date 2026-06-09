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

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

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

func FindTx[T any](ctx context.Context, tx Queryable, sql string, args ...any) (T, error) {
	var result T
	results, err := FindAllTx[T](ctx, tx, sql, args...)
	if err != nil {
		return result, fmt.Errorf("db find error: %w", err)
	}

	if len(results) == 0 {
		return result, fmt.Errorf("db find error: %w", ErrNotFound)
	}

	result = results[0]

	return result, nil
}
