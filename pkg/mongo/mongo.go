// Package mongo provides MongoDB connection lifecycle helpers.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

const defaultConnectTimeout = 10 * time.Second

// Database wraps a MongoDB client and selected database.
type Database struct {
	client *mongo.Client
	db     *mongo.Database
}

// Connect establishes a MongoDB connection and verifies connectivity.
func Connect(ctx context.Context, uri, database string) (*Database, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetTimeout(defaultConnectTimeout))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, defaultConnectTimeout)
	defer cancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		disconnectErr := client.Disconnect(context.WithoutCancel(ctx))
		if disconnectErr != nil {
			return nil, errors.Join(
				fmt.Errorf("mongo ping: %w", err),
				fmt.Errorf("mongo disconnect after ping failure: %w", disconnectErr),
			)
		}

		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	return &Database{client: client, db: client.Database(database)}, nil
}

// DB returns the selected database handle.
func (d *Database) DB() *mongo.Database {
	return d.db
}

// Close disconnects the MongoDB client.
func (d *Database) Close(ctx context.Context) error {
	if d == nil || d.client == nil {
		return nil
	}

	if err := d.client.Disconnect(ctx); err != nil {
		return fmt.Errorf("mongo disconnect: %w", err)
	}

	return nil
}
