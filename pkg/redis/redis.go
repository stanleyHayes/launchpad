// Package redisx provides Redis connection lifecycle helpers.
package redisx

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultPingTimeout = 5 * time.Second

// Client wraps a Redis client.
type Client struct {
	rdb *redis.Client
}

// Connect establishes a Redis connection and verifies connectivity.
func Connect(ctx context.Context, redisURL string) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis parse url: %w", err)
	}

	rdb := redis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()

	if err := rdb.Ping(pingCtx).Err(); err != nil {
		closeErr := rdb.Close()
		if closeErr != nil {
			return nil, errors.Join(
				fmt.Errorf("redis ping: %w", err),
				fmt.Errorf("redis close after ping failure: %w", closeErr),
			)
		}

		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// RDB returns the underlying go-redis client.
func (c *Client) RDB() *redis.Client {
	return c.rdb
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}

	if err := c.rdb.Close(); err != nil {
		return fmt.Errorf("redis close: %w", err)
	}

	return nil
}
