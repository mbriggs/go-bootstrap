package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func InTx[T any](ctx context.Context, fn func(tx pgx.Tx) (T, error)) (T, error) {
	var result T

	tx, err := Conn.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("could not open transaction: %w", err)
	}

	result, err = fn(tx)

	if err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			logger.Error("could not roll back transaction", "error", rbErr)
		}

		return result, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return result, fmt.Errorf("error committing transaction: %w", err)
	}

	return result, nil
}

func ExecInTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := Conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("could not open transaction: %w", err)
	}

	err = fn(tx)

	if err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			logger.Error("could not roll back transaction", "error", rbErr)
		}

		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}
