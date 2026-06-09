package webtest

import (
	"context"
	"sync"

	"github.com/labstack/echo/v4"

	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/env"
	"github.com/mbriggs/go-bootstrap/web"
)

var configureWeb sync.Once

// Server returns an Echo app wired like production — sessions backed by the
// package's test database, the full middleware stack, and all routes. Use
// with NewClient for session-bound flows (signin, flash, redirects).
// Safe under t.Parallel: the web package is configured once per test
// process.
func Server(ctx context.Context) *echo.Echo {
	configureWeb.Do(func() { web.Configure(db.Conn, env.Test) })
	return web.Router(ctx, "public")
}
