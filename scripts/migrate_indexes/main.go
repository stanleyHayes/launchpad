// Command migrate_indexes creates required MongoDB indexes.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"launchpad/internal/assignments"
	"launchpad/internal/audit"
	"launchpad/internal/auth"
	"launchpad/internal/departments"
	"launchpad/internal/employees"
	"launchpad/internal/journeys"
	"launchpad/internal/notifications"
	"launchpad/internal/organizations"
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

	db := mongoDB.DB()
	indexers := []struct {
		name string
		fn   func(context.Context) error
	}{
		{name: "audit", fn: audit.NewStore(db).EnsureIndexes},
		{name: "organization", fn: organizations.NewStore(db).EnsureIndexes},
		{name: "user", fn: auth.NewUserStore(db).EnsureIndexes},
		{name: "department", fn: departments.NewStore(db).EnsureIndexes},
		{name: "employee", fn: employees.NewStore(db).EnsureIndexes},
		{name: "journey", fn: journeys.NewStore(db).EnsureIndexes},
		{name: "assignment", fn: assignments.NewStore(db).EnsureIndexes},
		{name: "notification", fn: notifications.NewStore(db).EnsureIndexes},
	}

	for _, indexer := range indexers {
		if err := indexer.fn(ctx); err != nil {
			return fmt.Errorf("ensure %s indexes: %w", indexer.name, err)
		}
	}

	slog.Info("mongodb indexes ensured")

	return nil
}
