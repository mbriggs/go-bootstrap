package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/mbriggs/gesso/ui"
	"github.com/mbriggs/go-bootstrap/logging"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Router builds the production handler chain. Configure must have run
// first — sessions back the flash, CSRF-adjacent same-origin gate, and
// signed-in user loading.
func Router(ctx context.Context, publicDir string) *echo.Echo {
	if Sessions == nil {
		panic("web.Router: call web.Configure before building the router")
	}

	e := echo.New()
	e.HTTPErrorHandler = errorHandler

	// Echo's default RealIP trusts X-Forwarded-For from anyone, which lets a
	// client rotate the signin-throttle key by spoofing the header. Trust the
	// socket address only; behind a reverse proxy, swap in
	// echo.ExtractIPFromXFFHeader(echo.TrustIPRange(...)) for the proxy's range.
	e.IPExtractor = echo.ExtractIPDirect()

	// No CORS layer: sessions are same-origin and SameOriginPost gates
	// writes. v5 makes permissive CORS an explicit choice — when the app
	// grows cross-origin consumers, add middleware.CORSWithConfig with the
	// allowed origins spelled out.
	e.Use(middleware.RequestID())
	// Request spans no-op until telemetry.Configure installs a provider.
	e.Use(Tracing)
	e.Use(requestIDOnSpan)
	e.Use(middleware.Recover())
	e.Use(middleware.RequestLoggerWithConfig(loggerConfig()))
	e.Use(echo.WrapMiddleware(Sessions.LoadAndSave))
	e.Use(SameOriginPost)
	e.Use(LoadUser)

	e.Static("/public", publicDir)
	e.GET("/ui/*", echo.WrapHandler(http.StripPrefix("/ui/", ui.Assets())))

	// /health is liveness (process up); /ready is readiness (Postgres
	// reachable) — point load balancers and orchestrators at /ready.
	e.GET("/health", func(c *echo.Context) error {
		return c.String(200, "A-OK!")
	})

	e.GET("/ready", func(c *echo.Context) error {
		pingCtx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(pingCtx); err != nil {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "database unreachable")
		}

		return c.String(200, "ready")
	})

	e.GET("/design", DesignShowcase)

	e.GET("/signin", SigninForm)
	e.POST("/signin", SigninSubmit)
	e.POST("/signout", Signout)

	e.GET("/password-reset", PasswordResetRequestForm)
	e.POST("/password-reset", PasswordResetRequest)
	e.GET("/password-reset/confirm", PasswordResetConfirmForm)
	e.POST("/password-reset/confirm", PasswordResetConfirm)

	e.GET("/", Home, RequireUser)

	return e
}

// requestIDOnSpan puts the request id on the request span, so a
// client-reported X-Request-Id finds its trace. The reverse direction —
// trace ids on log lines — is the logging package's traceHandler.
func requestIDOnSpan(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if rid := c.Response().Header().Get(echo.HeaderXRequestID); rid != "" {
			trace.SpanFromContext(c.Request().Context()).
				SetAttributes(attribute.String("http.request_id", rid))
		}

		return next(c)
	}
}

func loggerConfig() middleware.RequestLoggerConfig {
	logger := logging.Logger("request")
	return middleware.RequestLoggerConfig{
		LogStatus:    true,
		LogLatency:   true,
		LogMethod:    true,
		LogURI:       true,
		LogRequestID: true,
		HandleError:  true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			// The request context carries the request span, so these
			// lines pick up trace_id/span_id via logging's traceHandler.
			ctx := c.Request().Context()
			if v.Error == nil {
				logger.LogAttrs(
					ctx, slog.LevelInfo, fmt.Sprintf("%s %s", v.Method, v.URI),
					slog.Int("status", v.Status),
					slog.Duration("latency", v.Latency),
					slog.String("request_id", v.RequestID),
				)
			} else {
				logger.LogAttrs(
					ctx, slog.LevelError, "REQUEST_ERROR",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("err", v.Error.Error()),
					slog.String("request_id", v.RequestID),
				)
			}
			return nil
		},
	}
}
