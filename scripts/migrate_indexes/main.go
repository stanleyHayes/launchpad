// Command migrate_indexes creates required MongoDB indexes.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"launchpad/internal/app"
	"launchpad/pkg/config"
	"launchpad/pkg/logging"
	mongox "launchpad/pkg/mongo"
)

const migrateTimeout = 30 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("migrate indexes failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logging.Setup(cfg.AppEnv)

	ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
	defer cancel()

	mongoDB, err := mongox.Connect(ctx, cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		return fmt.Errorf("connect to MongoDB: %w", err)
	}
	defer func() {
		if closeErr := mongoDB.Close(context.WithoutCancel(ctx)); closeErr != nil {
			slog.Error("mongo close failed", "error", closeErr)
		}
	}()

	for _, indexer := range app.MongoIndexers(mongoDB.DB()) {
		if err := indexer.Ensure(ctx); err != nil {
			return fmt.Errorf("ensure %s indexes: %w", indexer.Name, err)
		}
	}

	slog.Info("mongodb indexes ensured")

	return nil
}
