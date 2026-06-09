package db

import (
	"fmt"

	"github.com/jackc/pgx/v5"
)

func Ids(rows pgx.Rows) ([]int64, error) {
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64

		err := rows.Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("error scanning id: %w", err)
		}

		ids = append(ids, id)
	}

	if rows.Err() != nil {
		logger.Error("query error", "error", rows.Err())
		return nil, fmt.Errorf("query error: %w", rows.Err())
	}

	return ids, nil
}

func Id(rows pgx.Rows) (int64, error) {
	ids, err := Ids(rows)

	if err != nil {
		return 0, err
	}

	if len(ids) == 0 {
		return 0, fmt.Errorf("no ids returned")
	}

	if len(ids) != 1 {
		return 0, fmt.Errorf("multiple ids returned, expected only one: %v", ids)
	}

	return ids[0], nil
}
