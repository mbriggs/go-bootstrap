// Package webtest is the integration test harness. Each test package gets
// its own database cloned from the migrated template database, so packages
// are isolated from each other and from dev data, and run in parallel as
// separate processes. Within a package, tests that only touch rows they
// created (unique fixture names) may call t.Parallel(); tests that assert
// on table-wide state must stay serial.
//
// Database names are derived from the module path at runtime
// (<project>_template, <project>_test_<pid>), so bootstrapped projects
// need no renaming here.
package webtest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
	"github.com/mbriggs/go-bootstrap/appname"
	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/dev"
	"github.com/mbriggs/go-bootstrap/logging"
)

var Echo = echo.New()

var logger = logging.Logger("webtest")

var templateDB = templateDBName()

// templateDBName honors the TEMPLATE_DB override written by
// bin/worktree-setup, so each worktree clones from its own template.
func templateDBName() string {
	if name := os.Getenv("TEMPLATE_DB"); name != "" {
		return name
	}

	return projectName() + "_template"
}

// Main wraps testing.M for packages that touch the database. Call it from
// TestMain:
//
//	func TestMain(m *testing.M) { webtest.Main(m) }
func Main(m *testing.M) {
	code, err := run(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "webtest:", err)
		os.Exit(1)
	}

	os.Exit(code)
}

func run(m *testing.M) (int, error) {
	ctx := context.Background()
	name := fmt.Sprintf("%s_test_%d", projectName(), os.Getpid())

	if err := createTestDB(ctx, name); err != nil {
		return 0, err
	}
	defer dropTestDB(name)

	if err := db.Configure(ctx, "dbname="+name); err != nil {
		return 0, fmt.Errorf("configuring db %s: %w", name, err)
	}
	defer db.Close()

	dev.DevMode()

	settings := os.Getenv("TEST_LOGGING")
	if settings == "" {
		settings = "_all:warn"
	}

	if err := logging.Configure(settings, ""); err != nil {
		return 0, fmt.Errorf("configuring logging: %w", err)
	}

	return m.Run(), nil
}

// projectName is appname.Postgres with fail-loud semantics: silently wrong
// database names would break test isolation.
func projectName() string {
	name := appname.Postgres()
	if name == "" {
		panic("webtest: no module build info to derive project name from")
	}

	return name
}

func createTestDB(ctx context.Context, name string) error {
	admin, err := pgx.Connect(ctx, "dbname=postgres")
	if err != nil {
		return fmt.Errorf("connecting to admin db: %w", err)
	}
	defer admin.Close(ctx)

	// Database names can't be parameters in DDL; Sanitize quotes them so a
	// surprising module path can't produce mangled SQL.
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()); err != nil {
		return fmt.Errorf("dropping stale test db %s: %w", name, err)
	}

	// Two transient failures are expected and retried: parallel test
	// packages clone concurrently and Postgres rejects a clone while
	// another holds the template (SQLSTATE 55006), and bin/testdb's
	// drop-and-rename swap leaves the template absent for a few
	// milliseconds. Anything that persists through the retries is real.
	for attempt := 0; ; attempt++ {
		_, err = admin.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s",
			pgx.Identifier{name}.Sanitize(), pgx.Identifier{templateDB}.Sanitize()))

		if err == nil {
			return nil
		}

		if attempt >= 20 {
			if strings.Contains(err.Error(), "does not exist") {
				return fmt.Errorf("template database %s missing — run bin/testdb first: %w", templateDB, err)
			}

			return fmt.Errorf("creating test db %s from template: %w", name, err)
		}

		time.Sleep(250 * time.Millisecond)
	}
}

func dropTestDB(name string) {
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, "dbname=postgres")
	if err != nil {
		logger.Error("could not connect to drop test db", "db", name, "error", err)
		return
	}
	defer admin.Close(ctx)

	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()); err != nil {
		logger.Error("could not drop test db", "db", name, "error", err)
	}
}
