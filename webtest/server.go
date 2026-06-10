package webtest

import (
	"context"
	"sync"

	"github.com/labstack/echo/v5"

	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/env"
	"github.com/mbriggs/go-bootstrap/jobs"
	"github.com/mbriggs/go-bootstrap/web"
)

var configureWeb sync.Once

// TestBaseURL is the origin handlers and workers see in tests — emailed
// links in enqueued job args start with it.
const TestBaseURL = "http://test.localhost"

// Server returns an Echo app wired like production — sessions backed by the
// package's test database, the full middleware stack, and all routes. The
// jobs client is configured but never started: handlers can enqueue, tests
// can assert on the enqueued rows, and no worker runs underneath them.
// Safe under t.Parallel: the web package is configured once per test
// process.
func Server(ctx context.Context) *echo.Echo {
	// ensure the web package is configured once per test process.
	configureWeb.Do(func() {
		web.Configure(db.Conn, env.Test)

		if err := jobs.Configure(db.Conn, TestBaseURL); err != nil {
			panic("webtest: configuring jobs: " + err.Error())
		}
	})

	return web.Router(ctx, "public")
}
