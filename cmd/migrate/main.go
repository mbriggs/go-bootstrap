// Command migrate runs goose against the database the PG* environment
// variables point at — the same variables pgx reads, so .env (or
// worktree.env) decides which database, and bin/testdb retargets the
// template database by overriding PGDATABASE. Migrations live in
// ./migrations; goose is a go.mod dependency, not an installed binary.
//
// Usage:
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate status
//	go run ./cmd/migrate create add_things_table
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const dir = "migrations"

var errUsage = errors.New("usage: migrate <up|up-by-one|down|redo|reset|status|version> | migrate create <name>")

func main() {
	if err := mainErr(); err != nil {
		if errors.Is(err, errUsage) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		log.Fatal(err)
	}
}

func mainErr() error {
	if len(os.Args) < 2 {
		return errUsage
	}
	cmd := os.Args[1]

	if cmd == "create" {
		if len(os.Args) != 3 {
			return errUsage
		}
		if err := goose.Create(nil, dir, os.Args[2], "sql"); err != nil {
			return fmt.Errorf("create migration: %w", err)
		}
		return nil
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	// An empty DSN makes pgx build the connection entirely from PG* vars.
	db, err := sql.Open("pgx", "")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	return run(context.Background(), db, cmd)
}

func run(ctx context.Context, db *sql.DB, cmd string) error {
	var err error

	switch cmd {
	case "up":
		err = goose.UpContext(ctx, db, dir)
	case "up-by-one":
		err = goose.UpByOneContext(ctx, db, dir)
	case "down":
		err = goose.DownContext(ctx, db, dir)
	case "redo":
		err = goose.RedoContext(ctx, db, dir)
	case "reset":
		err = goose.ResetContext(ctx, db, dir)
	case "status":
		err = goose.StatusContext(ctx, db, dir)
	case "version":
		err = goose.VersionContext(ctx, db, dir)
	default:
		return errUsage
	}

	if err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	return nil
}
