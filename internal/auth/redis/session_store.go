// Package redis is the Redis persistence adapter for auth sessions.
package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"launchpad/internal/auth"
)

const sessionPayloadPartsExpected = 3

var _ auth.SessionRepository = (*SessionStore)(nil)

// SessionStore persists refresh sessions in Redis.
type SessionStore struct {
	rdb *goredis.Client
	ttl time.Duration
}

// NewSessionStore constructs a SessionStore.
func NewSessionStore(rdb *goredis.Client, ttl time.Duration) *SessionStore {
	return &SessionStore{rdb: rdb, ttl: ttl}
}

// Save stores a session payload and indexes the session under its user so all
// of a user's sessions can be revoked at once (password reset).
func (s *SessionStore) Save(ctx context.Context, sessionID, userID, orgID, refreshHash string) error {
	payload := strings.Join([]string{userID, orgID, refreshHash}, "|")

	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, s.key(sessionID), payload, s.ttl)
	pipe.SAdd(ctx, s.userKey(userID), sessionID)
	pipe.Expire(ctx, s.userKey(userID), s.ttl)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	return nil
}

// Get loads a session payload.
func (s *SessionStore) Get(ctx context.Context, sessionID string) (string, string, string, error) {
	val, getErr := s.rdb.Get(ctx, s.key(sessionID)).Result()
	if errors.Is(getErr, goredis.Nil) {
		return "", "", "", auth.ErrSessionInvalid
	}

	if getErr != nil {
		return "", "", "", fmt.Errorf("get session: %w", getErr)
	}

	parts := strings.Split(val, "|")
	if len(parts) != sessionPayloadPartsExpected {
		return "", "", "", auth.ErrSessionInvalid
	}

	return parts[0], parts[1], parts[2], nil
}

// Exists reports whether a session with the given ID is present.
func (s *SessionStore) Exists(ctx context.Context, sessionID string) (bool, error) {
	count, err := s.rdb.Exists(ctx, s.key(sessionID)).Result()
	if err != nil {
		return false, fmt.Errorf("check session: %w", err)
	}

	return count > 0, nil
}

// Delete removes a session. The per-user index entry is left to expire with
// the set; DeleteForUser tolerates stale members (deleting a missing session
// key is a no-op).
func (s *SessionStore) Delete(ctx context.Context, sessionID string) error {
	if err := s.rdb.Del(ctx, s.key(sessionID)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// DeleteForUser revokes every session belonging to a user.
func (s *SessionStore) DeleteForUser(ctx context.Context, userID string) error {
	sessionIDs, err := s.rdb.SMembers(ctx, s.userKey(userID)).Result()
	if err != nil {
		return fmt.Errorf("list user sessions: %w", err)
	}

	keys := make([]string, 0, len(sessionIDs)+1)
	for _, sessionID := range sessionIDs {
		keys = append(keys, s.key(sessionID))
	}

	keys = append(keys, s.userKey(userID))

	if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}

	return nil
}

func (s *SessionStore) key(sessionID string) string {
	return "session:" + sessionID
}

func (s *SessionStore) userKey(userID string) string {
	return "session:user:" + userID
}
