// Package db is the generic data-access mechanism: typed queries over any
// Queryable (pool or transaction), transaction boundaries, and row-map
// helpers. Domain packages own their SQL; this package owns how SQL runs.
// The pool global Conn is confined by lint to generated conn.gen.go files,
// this package's bootstrap, and package main.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mbriggs/go-bootstrap/logging"
)

var logger = logging.Logger("db")

// Conn is the database connection pool. Configure must be called before using
var Conn *pgxpool.Pool

var ErrNotFound = errors.New("not found")

// Configure sets up the database connection pool. If dsn is "", PG ENV vars will be used.
func Configure(ctx context.Context, dsn string) error {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parsing pool config: %w", err)
	}

	// Query spans no-op until telemetry.Configure installs a provider.
	cfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("creating connection pool: %w", err)
	}

	// Ping the database to ensure the connection is working.
	err = pool.Ping(ctx)
	if err != nil {
		return fmt.Errorf("error pinging database: %w", err)
	}

	Conn = pool

	return nil
}

// Close releases the connection pool.
func Close() {
	Conn.Close()
}
