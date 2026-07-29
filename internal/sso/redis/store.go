// Package redis is the Redis persistence adapter for short-lived SSO login state.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"launchpad/internal/sso"
)

const keyPrefix = "sso:state:"

var _ sso.StateStore = (*Store)(nil)

// Store persists login state in Redis with a short TTL.
type Store struct {
	rdb *goredis.Client
}

// NewStore constructs a Store.
func NewStore(rdb *goredis.Client) *Store {
	return &Store{rdb: rdb}
}

// Save stores login state under the opaque state key with a TTL.
func (s *Store) Save(ctx context.Context, state string, data sso.AuthState, ttl time.Duration) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal sso state: %w", err)
	}

	if err := s.rdb.Set(ctx, keyPrefix+state, payload, ttl).Err(); err != nil {
		return fmt.Errorf("save sso state: %w", err)
	}

	return nil
}

// Consume atomically reads and deletes the login state so it can be used once.
func (s *Store) Consume(ctx context.Context, state string) (sso.AuthState, error) {
	payload, err := s.rdb.GetDel(ctx, keyPrefix+state).Result()
	if errors.Is(err, goredis.Nil) {
		return sso.AuthState{}, sso.ErrInvalidState
	}

	if err != nil {
		return sso.AuthState{}, fmt.Errorf("consume sso state: %w", err)
	}

	var data sso.AuthState
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return sso.AuthState{}, fmt.Errorf("decode sso state: %w", err)
	}

	return data, nil
}
