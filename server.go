package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/env"
	"github.com/mbriggs/go-bootstrap/flows"
	"github.com/mbriggs/go-bootstrap/jobs"
	"github.com/mbriggs/go-bootstrap/logging"
	"github.com/mbriggs/go-bootstrap/telemetry"
	"github.com/mbriggs/go-bootstrap/web"
)

func main() {
	if err := run(); err != nil {
		logging.Logger("bootstrap").Error("fatal", slog.Any("err", err))
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	logger := logging.Logger("bootstrap")

	cfg, err := env.Load()
	if err != nil {
		return err
	}

	if err := logging.Configure("_all", "info"); err != nil {
		return err
	}

	traceShutdown, err := telemetry.Configure(ctx)
	if err != nil {
		return fmt.Errorf("configuring telemetry: %w", err)
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := traceShutdown(flushCtx); err != nil {
			logger.Error("flushing traces", slog.Any("err", err))
		}
	}()

	if err := db.Configure(ctx, ""); err != nil {
		return err
	}
	defer db.Close()

	web.Configure(db.Conn, cfg.AppEnv)

	if err := jobs.Configure(db.Conn, cfg.BaseURL); err != nil {
		return fmt.Errorf("configuring jobs: %w", err)
	}
	if err := jobs.Start(ctx); err != nil {
		return fmt.Errorf("starting jobs: %w", err)
	}

	e := web.Router(ctx, cfg.PublicDir)

	// Flows mount in main, not web.Router: the route exists only where the
	// Inngest server can actually call back (not in webtest), and web stays
	// uncoupled from the orchestration tier.
	flowsHandler, err := flows.Configure()
	if err != nil {
		return fmt.Errorf("configuring flows: %w", err)
	}
	e.Any("/api/inngest", echo.WrapHandler(flowsHandler))

	// Bind localhost in dev so the server isn't exposed on the LAN; bind
	// all interfaces in production where something routes to us.
	addr := ":" + cfg.Port
	if cfg.Dev() {
		addr = "localhost:" + cfg.Port
	}

	// Full timeout set, not just headers — without ReadTimeout/WriteTimeout a
	// slow-body client holds a connection open indefinitely.
	srv := &http.Server{
		Addr:              addr,
		Handler:           e,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	drained := make(chan struct{})
	go func() {
		sigC := make(chan os.Signal, 1)
		signal.Notify(sigC, os.Interrupt, syscall.SIGTERM)
		<-sigC
		logger.Info("shutdown: draining connections")

		shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown", slog.Any("err", err))
		}
		close(drained)
	}()

	logger.Info("listening", slog.String("addr", addr))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}
	<-drained

	// HTTP is drained; now drain the job workers.
	stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := jobs.Stop(stopCtx); err != nil {
		logger.Error("stopping jobs", slog.Any("err", err))
	}

	return nil
}
