package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// SessionStore persists refresh sessions in Redis.
type SessionStore struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewSessionStore constructs a SessionStore.
func NewSessionStore(rdb *redis.Client, ttl time.Duration) *SessionStore {
	return &SessionStore{rdb: rdb, ttl: ttl}
}

// Save stores a session payload.
func (s *SessionStore) Save(ctx context.Context, sessionID, userID, orgID, refreshHash string) error {
	payload := strings.Join([]string{userID, orgID, refreshHash}, "|")
	if err := s.rdb.Set(ctx, s.key(sessionID), payload, s.ttl).Err(); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	return nil
}

// Get loads a session payload.
func (s *SessionStore) Get(ctx context.Context, sessionID string) (string, string, string, error) {
	val, getErr := s.rdb.Get(ctx, s.key(sessionID)).Result()
	if errors.Is(getErr, redis.Nil) {
		return "", "", "", ErrSessionInvalid
	}

	if getErr != nil {
		return "", "", "", fmt.Errorf("get session: %w", getErr)
	}

	parts := strings.Split(val, "|")
	if len(parts) != sessionPayloadPartsExpected {
		return "", "", "", ErrSessionInvalid
	}

	return parts[0], parts[1], parts[2], nil
}

// Delete removes a session.
func (s *SessionStore) Delete(ctx context.Context, sessionID string) error {
	if err := s.rdb.Del(ctx, s.key(sessionID)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

func (s *SessionStore) key(sessionID string) string {
	return "session:" + sessionID
}
