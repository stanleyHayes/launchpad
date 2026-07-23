// Command migrate_indexes creates required MongoDB indexes.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	assignmentsmongo "launchpad/internal/assignments/mongo"
	auditmongo "launchpad/internal/audit/mongo"
	authmongo "launchpad/internal/auth/mongo"
	billingmongo "launchpad/internal/billing/mongo"
	cmsmongo "launchpad/internal/cms/mongo"
	departmentsmongo "launchpad/internal/departments/mongo"
	employeesmongo "launchpad/internal/employees/mongo"
	featureflagsmongo "launchpad/internal/featureflags/mongo"
	journeysmongo "launchpad/internal/journeys/mongo"
	leadsmongo "launchpad/internal/leads/mongo"
	notificationsmongo "launchpad/internal/notifications/mongo"
	organizationsmongo "launchpad/internal/organizations/mongo"
	platformmongo "launchpad/internal/platform/mongo"
	supportmongo "launchpad/internal/support/mongo"
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
		{name: "audit", fn: auditmongo.NewStore(db).EnsureIndexes},
		{name: "organization", fn: organizationsmongo.NewStore(db).EnsureIndexes},
		{name: "user", fn: authmongo.NewUserStore(db).EnsureIndexes},
		{name: "department", fn: departmentsmongo.NewStore(db).EnsureIndexes},
		{name: "employee", fn: employeesmongo.NewStore(db).EnsureIndexes},
		{name: "journey", fn: journeysmongo.NewStore(db).EnsureIndexes},
		{name: "assignment", fn: assignmentsmongo.NewStore(db).EnsureIndexes},
		{name: "notification", fn: notificationsmongo.NewStore(db).EnsureIndexes},
		{name: "platform", fn: platformmongo.NewStore(db).EnsureIndexes},
		{name: "leads", fn: leadsmongo.NewStore(db).EnsureIndexes},
		{name: "featureflags", fn: featureflagsmongo.NewStore(db).EnsureIndexes},
		{name: "billing", fn: billingmongo.NewStore(db).EnsureIndexes},
		{name: "support", fn: supportmongo.NewStore(db).EnsureIndexes},
		{name: "cms", fn: cmsmongo.NewStore(db).EnsureIndexes},
	}

	for _, indexer := range indexers {
		if err := indexer.fn(ctx); err != nil {
			return fmt.Errorf("ensure %s indexes: %w", indexer.name, err)
		}
	}

	slog.Info("mongodb indexes ensured")

	return nil
}
