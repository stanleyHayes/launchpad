package auth

import "context"

// UserRepository persists users.
type UserRepository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, user User) error
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
}

// SessionRepository persists refresh sessions.
type SessionRepository interface {
	Save(ctx context.Context, sessionID, userID, orgID, refreshHash string) error
	Get(ctx context.Context, sessionID string) (userID, orgID, refreshHash string, err error)
	Delete(ctx context.Context, sessionID string) error
}
