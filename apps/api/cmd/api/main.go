// Command api starts the Launchpad API server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"launchpad/internal/app"
	"launchpad/pkg/config"
	"launchpad/pkg/logging"
	mongox "launchpad/pkg/mongo"
	redisx "launchpad/pkg/redis"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() (err error) {
	cfg, loadErr := config.Load()
	if loadErr != nil {
		return fmt.Errorf("load configuration: %w", loadErr)
	}

	logging.Setup(cfg.AppEnv)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mongoDB, mongoErr := mongox.Connect(ctx, cfg.MongoURI, cfg.MongoDatabase)
	if mongoErr != nil {
		return fmt.Errorf("connect to MongoDB: %w", mongoErr)
	}
	defer func() {
		closeErr := mongoDB.Close(context.WithoutCancel(ctx))
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	redisClient, redisErr := redisx.Connect(ctx, cfg.RedisURL)
	if redisErr != nil {
		return fmt.Errorf("connect to Redis: %w", redisErr)
	}
	defer func() {
		closeErr := redisClient.Close()
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()

	runErr := app.Run(ctx, cfg, app.Dependencies{
		Mongo: mongoDB,
		Redis: redisClient,
	})
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		return fmt.Errorf("run application: %w", runErr)
	}

	slog.Info("api shutdown complete")

	return nil
}
