package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/mbriggs/gesso/ui"
	"github.com/mbriggs/go-bootstrap/logging"
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

	e.Use(middleware.CORS())
	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestLoggerWithConfig(loggerConfig()))
	e.Use(echo.WrapMiddleware(Sessions.LoadAndSave))
	e.Use(SameOriginPost)
	e.Use(LoadUser)

	e.Static("/public", publicDir)
	e.GET("/ui/*", echo.WrapHandler(http.StripPrefix("/ui/", ui.Assets())))

	e.GET("/health", func(c echo.Context) error {
		return c.String(200, "A-OK!")
	})

	e.GET("/design", DesignShowcase)

	e.GET("/signin", SigninForm)
	e.POST("/signin", SigninSubmit)
	e.POST("/signout", Signout)

	e.GET("/", Home, RequireUser)

	return e
}

func loggerConfig() middleware.RequestLoggerConfig {
	logger := logging.Logger("request")
	return middleware.RequestLoggerConfig{
		LogStatus:   true,
		LogLatency:  true,
		LogMethod:   true,
		LogURI:      true,
		LogError:    true,
		HandleError: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error == nil {
				logger.LogAttrs(
					context.Background(), slog.LevelInfo, fmt.Sprintf("%s %s", v.Method, v.URI),
					slog.Int("status", v.Status),
					slog.Duration("latency", v.Latency),
				)
			} else {
				logger.LogAttrs(
					context.Background(), slog.LevelError, "REQUEST_ERROR",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("err", v.Error.Error()),
				)
			}
			return nil
		},
	}
}
