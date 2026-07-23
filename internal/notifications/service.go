package notifications

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service implements notification use cases.
type Service struct {
	store *Store
}

// NewService constructs a Service.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// Create creates a notification.
func (s *Service) Create(ctx context.Context, organizationID string, in CreateInput) (Notification, error) {
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)

	userID := strings.TrimSpace(in.UserID)
	if organizationID == "" || userID == "" || title == "" || body == "" {
		return Notification{}, ErrInvalidInput
	}

	notification := Notification{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		UserID:         userID,
		Title:          title,
		Body:           body,
		ReadAt:         nil,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.store.Create(ctx, notification); err != nil {
		return Notification{}, fmt.Errorf("create notification: %w", err)
	}

	return notification, nil
}

// ListForUser lists notifications for a user.
func (s *Service) ListForUser(ctx context.Context, organizationID, userID string) ([]Notification, error) {
	if organizationID == "" || userID == "" {
		return nil, ErrInvalidInput
	}

	items, err := s.store.ListForUser(ctx, organizationID, userID)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	return items, nil
}

// MarkRead marks a notification as read.
func (s *Service) MarkRead(ctx context.Context, organizationID, userID, notificationID string) (Notification, error) {
	if organizationID == "" || userID == "" || notificationID == "" {
		return Notification{}, ErrInvalidInput
	}

	notification, err := s.store.Get(ctx, organizationID, userID, notificationID)
	if err != nil {
		return Notification{}, fmt.Errorf("get notification: %w", err)
	}

	now := time.Now().UTC()

	notification.ReadAt = &now
	if err := s.store.Update(ctx, notification); err != nil {
		return Notification{}, fmt.Errorf("mark notification read: %w", err)
	}

	return notification, nil
}
