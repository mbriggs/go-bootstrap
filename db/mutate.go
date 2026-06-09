package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/mbriggs/pgsql"
)

var ErrIncorrectMutation = errors.New("incorrect mutation")
var ErrMutationScope = errors.New("incorrect number of rows mutated")

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
