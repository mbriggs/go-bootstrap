package main

import (
	"context"
	"os"

	"log/slog"

	"github.com/mbriggs/go-bootstrap/db"
	"github.com/mbriggs/go-bootstrap/logging"
	"github.com/mbriggs/go-bootstrap/web"
)

func main() {
	ctx := context.Background()
	logger := logging.Logger("bootstrap")

	err := logging.Configure("_all", "info")
	if err != nil {
		logger.Error("logging config error", slog.Any("err", err))
		os.Exit(1)
	}

	err = db.Configure(ctx, "")
	if err != nil {
		logger.Error("db config error", slog.Any("err", err))
		os.Exit(1)
	}

	e := web.Router(ctx)

	e.Logger.Fatal(e.Start("localhost:8080"))
}
