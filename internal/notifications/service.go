package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service implements notification use cases.
type Service struct {
	repo       Repository
	channels   ChannelStore
	dispatcher Dispatcher
	email      EmailDispatcher
	sms        SMSDispatcher
	deliveries DeliveryStore
}

func (s *Service) WithSMSDispatcher(dispatcher SMSDispatcher) *Service {
	s.sms = dispatcher
	return s
}

func (s *Service) WithDeliveryStore(store DeliveryStore) *Service {
	s.deliveries = store
	return s
}

func (s *Service) WithEmailDispatcher(dispatcher EmailDispatcher) *Service {
	s.email = dispatcher
	return s
}

// NewService constructs a Service.
func NewService(repo Repository, channels ChannelStore, dispatcher Dispatcher) *Service {
	return &Service{repo: repo, channels: channels, dispatcher: dispatcher}
}

// Create creates a notification.
func (s *Service) Create(ctx context.Context, organizationID string, in CreateInput) (Notification, error) {
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	link := strings.TrimSpace(in.Link)

	userID := strings.TrimSpace(in.UserID)
	if organizationID == "" || userID == "" || title == "" || body == "" {
		return Notification{}, ErrInvalidInput
	}

	// The link is rendered as a navigation target; restricting it to
	// app-relative paths keeps a notification from redirecting off-app.
	if link != "" && !strings.HasPrefix(link, "/") {
		return Notification{}, ErrInvalidInput
	}

	notificationType := strings.TrimSpace(in.Type)
	if notificationType == "" {
		notificationType = TypeSystem
	}

	notification := Notification{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		UserID:         userID,
		Type:           notificationType,
		Title:          title,
		Body:           body,
		Link:           link,
		ReadAt:         nil,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, notification); err != nil {
		return Notification{}, fmt.Errorf("create notification: %w", err)
	}

	s.deliver(ctx, organizationID, notification)

	return notification, nil
}

// GetChannelConfig returns the tenant's channel configuration, or an empty
// config when none is set.
func (s *Service) GetChannelConfig(ctx context.Context, organizationID string) (ChannelConfig, error) {
	if organizationID == "" {
		return ChannelConfig{}, ErrInvalidInput
	}

	config, err := s.channels.GetChannels(ctx, organizationID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ChannelConfig{OrganizationID: organizationID}, nil
		}

		return ChannelConfig{}, fmt.Errorf("get channel config: %w", err)
	}

	return config, nil
}

// SetChannelConfig validates and stores the tenant's outbound webhook URLs.
func (s *Service) SetChannelConfig(
	ctx context.Context,
	organizationID string,
	in SetChannelsInput,
) (ChannelConfig, error) {
	if organizationID == "" {
		return ChannelConfig{}, ErrInvalidInput
	}

	config, err := s.channels.GetChannels(ctx, organizationID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return ChannelConfig{}, fmt.Errorf("load channel config: %w", err)
		}

		config = ChannelConfig{OrganizationID: organizationID}
	}

	if in.SlackWebhookURL != nil {
		slackURL, err := validateWebhookURL(*in.SlackWebhookURL, slackHostAllowed)
		if err != nil {
			return ChannelConfig{}, err
		}

		config.SlackWebhookURL = slackURL
	}

	if in.TeamsWebhookURL != nil {
		teamsURL, err := validateWebhookURL(*in.TeamsWebhookURL, teamsHostAllowed)
		if err != nil {
			return ChannelConfig{}, err
		}

		config.TeamsWebhookURL = teamsURL
	}

	config.OrganizationID = organizationID
	config.UpdatedAt = time.Now().UTC()

	if err := s.channels.SetChannels(ctx, config); err != nil {
		return ChannelConfig{}, fmt.Errorf("set channel config: %w", err)
	}

	return config, nil
}

// validateWebhookURL enforces https and an allowlisted host so an operator
// cannot point delivery at an internal or arbitrary address (SSRF). An empty
// value clears the channel.
func validateWebhookURL(raw string, hostAllowed func(string) bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", ErrInvalidInput
	}

	if !hostAllowed(strings.ToLower(parsed.Hostname())) {
		return "", ErrInvalidInput
	}

	return raw, nil
}

func slackHostAllowed(host string) bool {
	return host == "hooks.slack.com"
}

func teamsHostAllowed(host string) bool {
	return strings.HasSuffix(host, ".office.com") || strings.HasSuffix(host, ".logic.azure.com")
}

// ListForUser lists notifications for a user.
func (s *Service) ListForUser(ctx context.Context, organizationID, userID string) ([]Notification, error) {
	if organizationID == "" || userID == "" {
		return nil, ErrInvalidInput
	}

	items, err := s.repo.ListForUser(ctx, organizationID, userID)
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

	notification, err := s.repo.Get(ctx, organizationID, userID, notificationID)
	if err != nil {
		return Notification{}, fmt.Errorf("get notification: %w", err)
	}

	now := time.Now().UTC()

	notification.ReadAt = &now
	if err := s.repo.Update(ctx, notification); err != nil {
		return Notification{}, fmt.Errorf("mark notification read: %w", err)
	}

	return notification, nil
}

// deliver best-effort forwards a notification to the tenant's chat channels.
// External delivery must never fail the notification itself.
func (s *Service) deliver(ctx context.Context, organizationID string, notification Notification) {
	if s.email != nil {
		s.deliverChannel(ctx, notification, "email", func() error {
			return s.email.DispatchEmail(ctx, notification)
		})
	}
	if s.sms != nil {
		s.deliverChannel(ctx, notification, "sms", func() error {
			return s.sms.DispatchSMS(ctx, notification)
		})
	}
	config, err := s.channels.GetChannels(ctx, organizationID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			slog.WarnContext(ctx, "load notification channels", "error", err)
		}

		return
	}

	if config.SlackWebhookURL != "" || config.TeamsWebhookURL != "" {
		s.deliverChannel(ctx, notification, "chat_webhook", func() error {
			return s.dispatcher.Dispatch(ctx, config, notification)
		})
	}
}

func (s *Service) deliverChannel(
	ctx context.Context, notification Notification, channel string, dispatch func() error,
) {
	now := time.Now().UTC()
	delivery := Delivery{
		ID: uuid.NewString(), OrganizationID: notification.OrganizationID,
		NotificationID: notification.ID, UserID: notification.UserID, Channel: channel,
		Status: DeliveryPending, CreatedAt: now, UpdatedAt: now,
	}
	if s.deliveries != nil {
		if err := s.deliveries.CreateDelivery(ctx, delivery); err != nil {
			slog.WarnContext(ctx, "create notification delivery", "error", err)
		}
	}
	s.attemptDelivery(ctx, notification, &delivery, dispatch)
}

func (s *Service) attemptDelivery(
	ctx context.Context, notification Notification, delivery *Delivery, dispatch func() error,
) {
	delivery.Attempts++
	delivery.UpdatedAt = time.Now().UTC()
	if err := dispatch(); err != nil {
		delivery.LastError = err.Error()
		if delivery.Attempts >= 3 {
			delivery.Status, delivery.NextAttemptAt = DeliveryDeadLetter, nil
		} else {
			next := time.Now().UTC().Add(time.Duration(delivery.Attempts*5) * time.Minute)
			delivery.Status, delivery.NextAttemptAt = DeliveryRetrying, &next
		}
		slog.WarnContext(ctx, "dispatch notification channel", "channel", delivery.Channel, "error", err)
	} else {
		delivery.Status, delivery.LastError, delivery.NextAttemptAt = DeliveryDelivered, "", nil
	}
	if s.deliveries != nil {
		if err := s.deliveries.UpdateDelivery(ctx, *delivery); err != nil {
			slog.WarnContext(ctx, "update notification delivery", "error", err)
		}
	}
}

func (s *Service) ListDeliveries(ctx context.Context) ([]Delivery, error) {
	if s.deliveries == nil {
		return []Delivery{}, nil
	}
	return s.deliveries.ListDeliveries(ctx)
}

func (s *Service) RetryDueDeliveries(ctx context.Context) error {
	if s.deliveries == nil {
		return nil
	}
	items, err := s.deliveries.ListDueDeliveries(ctx, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("list due notification deliveries: %w", err)
	}
	for _, delivery := range items {
		if err := s.retryDelivery(ctx, delivery); err != nil {
			slog.WarnContext(ctx, "retry notification delivery", "deliveryId", delivery.ID, "error", err)
		}
	}
	return nil
}

func (s *Service) RetryDelivery(ctx context.Context, id string) (Delivery, error) {
	if s.deliveries == nil {
		return Delivery{}, ErrNotFound
	}
	delivery, err := s.deliveries.GetDelivery(ctx, id)
	if err != nil {
		return Delivery{}, err
	}
	if err := s.retryDelivery(ctx, delivery); err != nil {
		return Delivery{}, err
	}
	return s.deliveries.GetDelivery(ctx, id)
}

func (s *Service) retryDelivery(ctx context.Context, delivery Delivery) error {
	notification, err := s.repo.Get(ctx, delivery.OrganizationID, delivery.UserID, delivery.NotificationID)
	if err != nil {
		return fmt.Errorf("load notification for retry: %w", err)
	}
	delivery.Status, delivery.NextAttemptAt = DeliveryRetrying, nil
	switch delivery.Channel {
	case "email":
		if s.email == nil {
			return ErrInvalidInput
		}
		s.attemptDelivery(ctx, notification, &delivery, func() error { return s.email.DispatchEmail(ctx, notification) })
	case "chat_webhook":
		config, err := s.channels.GetChannels(ctx, delivery.OrganizationID)
		if err != nil {
			return err
		}
		s.attemptDelivery(ctx, notification, &delivery, func() error { return s.dispatcher.Dispatch(ctx, config, notification) })
	case "sms":
		if s.sms == nil {
			return ErrInvalidInput
		}
		s.attemptDelivery(ctx, notification, &delivery, func() error { return s.sms.DispatchSMS(ctx, notification) })
	default:
		return ErrInvalidInput
	}
	return nil
}
