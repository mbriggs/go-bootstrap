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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mbriggs/go-bootstrap/logging"
)

var logger = logging.Logger("db")

// Conn is the database connection pool. Configure must be called before using
var Conn *pgxpool.Pool

var (
	ErrNotFound    = errors.New("not found")
	ErrTooManyRows = errors.New("more than one row matched")
)

// Configure sets up the database connection pool. If dsn is "", PG ENV vars will be used.
func Configure(ctx context.Context, dsn string) error {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parsing pool config: %w", err)
	}

	// Query spans no-op until telemetry.Configure installs a provider.
	cfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())

	// Extension types aren't in pgx's default type map because their OIDs
	// vary per database, so they register per connection. Skipping when the
	// extension is absent is safe: a column of the type can't exist without
	// it. modelgen maps hstore columns to map[string]string.
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		registerHstore(ctx, conn)
		return nil
	}

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

// registerHstore registers the hstore codec against this database's OIDs.
// pgx's LoadType can't synthesize a codec for an arbitrary base type, so
// the codec is named explicitly. A quiet miss means the extension isn't
// installed in this database.
func registerHstore(ctx context.Context, conn *pgx.Conn) {
	var oid, arrayOID uint32
	err := conn.QueryRow(ctx,
		"SELECT oid, typarray FROM pg_type WHERE oid = to_regtype('hstore')").Scan(&oid, &arrayOID)
	if err != nil {
		return
	}

	hstore := &pgtype.Type{Name: "hstore", OID: oid, Codec: pgtype.HstoreCodec{}}
	conn.TypeMap().RegisterType(hstore)
	conn.TypeMap().RegisterType(&pgtype.Type{
		Name: "_hstore", OID: arrayOID,
		Codec: &pgtype.ArrayCodec{ElementType: hstore},
	})
}
