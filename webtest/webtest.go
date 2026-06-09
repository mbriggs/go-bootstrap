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
	"path"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/dev"
	"github.com/mbriggs/go-bootstrap/logging"
)

var Echo *echo.Echo = echo.New()

var logger = logging.Logger("webtest")

var templateDB = projectName() + "_template"

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

// projectName derives a Postgres-safe identifier from the module path, so
// the harness works unmodified in any project bootstrapped from this one.
func projectName() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Path == "" {
		panic("webtest: no module build info to derive project name from")
	}

	name := strings.ToLower(path.Base(info.Main.Path))

	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}

	return b.String()
}

func createTestDB(ctx context.Context, name string) error {
	admin, err := pgx.Connect(ctx, "dbname=postgres")
	if err != nil {
		return fmt.Errorf("connecting to admin db: %w", err)
	}
	defer admin.Close(ctx)

	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name); err != nil {
		return fmt.Errorf("dropping stale test db %s: %w", name, err)
	}

	// Parallel test packages clone the template concurrently, and Postgres
	// rejects a clone while another clone holds the template (SQLSTATE 55006),
	// so contention here is expected and retried rather than fatal.
	for attempt := 0; ; attempt++ {
		_, err = admin.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, templateDB))

		if err == nil {
			return nil
		}

		if strings.Contains(err.Error(), "does not exist") {
			return fmt.Errorf("template database %s missing — run bin/testdb first: %w", templateDB, err)
		}

		if attempt >= 20 {
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

	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name); err != nil {
		logger.Error("could not drop test db", "db", name, "error", err)
	}
}
