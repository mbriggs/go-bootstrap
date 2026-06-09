package web

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/mbriggs/go-bootstrap/logging"
)

func Router(ctx context.Context) *echo.Echo {
	e := echo.New()

	e.Use(middleware.CORS())
	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestLoggerWithConfig(loggerConfig()))

	e.GET("/health", func(c echo.Context) error {
		return c.String(200, "A-OK!")
	})

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
				logger.LogAttrs(context.Background(), slog.LevelInfo, fmt.Sprintf("%s %s", v.Method, v.URI),
					slog.Int("status", v.Status),
					slog.Duration("latency", v.Latency),
				)
			} else {
				logger.LogAttrs(context.Background(), slog.LevelError, "REQUEST_ERROR",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("err", v.Error.Error()),
				)
			}
			return nil
		},
	}
}
