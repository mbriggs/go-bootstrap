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

	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/env"
	"github.com/mbriggs/go-bootstrap/logging"
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

	if err := db.Configure(ctx, ""); err != nil {
		return err
	}
	defer db.Close()

	web.Configure(db.Conn, cfg.AppEnv)

	e := web.Router(ctx, cfg.PublicDir)

	// Bind localhost in dev so the server isn't exposed on the LAN; bind
	// all interfaces in production where something routes to us.
	addr := ":" + cfg.Port
	if cfg.Dev() {
		addr = "localhost:" + cfg.Port
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           e,
		ReadHeaderTimeout: 10 * time.Second,
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

	return nil
}
