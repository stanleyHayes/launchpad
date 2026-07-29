package notifications_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"launchpad/internal/notifications"
)

const orgID = "org-1"

// fakeRepo records created notifications.
type fakeRepo struct {
	created []notifications.Notification
}

func (f *fakeRepo) EnsureIndexes(context.Context) error { return nil }

func (f *fakeRepo) Create(_ context.Context, n notifications.Notification) error {
	f.created = append(f.created, n)

	return nil
}

func (f *fakeRepo) ListForUser(context.Context, string, string) ([]notifications.Notification, error) {
	return nil, nil
}

func (f *fakeRepo) Get(_ context.Context, organizationID, userID, id string) (notifications.Notification, error) {
	for _, item := range f.created {
		if item.OrganizationID == organizationID && item.UserID == userID && item.ID == id {
			return item, nil
		}
	}
	return notifications.Notification{}, notifications.ErrNotFound
}

func (f *fakeRepo) Update(context.Context, notifications.Notification) error { return nil }

// fakeChannels serves a preset config and records upserts.
type fakeChannels struct {
	config *notifications.ChannelConfig
	saved  *notifications.ChannelConfig
}

func (f *fakeChannels) EnsureIndexes(context.Context) error { return nil }

func (f *fakeChannels) GetChannels(_ context.Context, _ string) (notifications.ChannelConfig, error) {
	if f.config == nil {
		return notifications.ChannelConfig{}, notifications.ErrNotFound
	}

	return *f.config, nil
}

func (f *fakeChannels) SetChannels(_ context.Context, config notifications.ChannelConfig) error {
	stored := config
	f.saved = &stored

	return nil
}

// fakeDispatcher records the last dispatch.
type fakeDispatcher struct {
	dispatched *notifications.ChannelConfig
	err        error
}

func (f *fakeDispatcher) Dispatch(
	_ context.Context,
	config notifications.ChannelConfig,
	_ notifications.Notification,
) error {
	stored := config
	f.dispatched = &stored

	return f.err
}

type fakeDeliveryStore struct {
	items map[string]notifications.Delivery
}

func newFakeDeliveryStore() *fakeDeliveryStore {
	return &fakeDeliveryStore{items: map[string]notifications.Delivery{}}
}
func (f *fakeDeliveryStore) EnsureIndexes(context.Context) error { return nil }
func (f *fakeDeliveryStore) CreateDelivery(_ context.Context, d notifications.Delivery) error {
	f.items[d.ID] = d
	return nil
}
func (f *fakeDeliveryStore) UpdateDelivery(_ context.Context, d notifications.Delivery) error {
	f.items[d.ID] = d
	return nil
}
func (f *fakeDeliveryStore) GetDelivery(_ context.Context, id string) (notifications.Delivery, error) {
	d, ok := f.items[id]
	if !ok {
		return notifications.Delivery{}, notifications.ErrNotFound
	}
	return d, nil
}
func (f *fakeDeliveryStore) ListDeliveries(context.Context) ([]notifications.Delivery, error) {
	out := make([]notifications.Delivery, 0, len(f.items))
	for _, d := range f.items {
		out = append(out, d)
	}
	return out, nil
}
func (f *fakeDeliveryStore) ListDueDeliveries(_ context.Context, now time.Time) ([]notifications.Delivery, error) {
	out := []notifications.Delivery{}
	for _, d := range f.items {
		if d.NextAttemptAt != nil && !d.NextAttemptAt.After(now) {
			out = append(out, d)
		}
	}
	return out, nil
}
func (f *fakeDeliveryStore) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func createInput() notifications.CreateInput {
	return notifications.CreateInput{UserID: "user-1", Title: "Welcome", Body: "Start your onboarding."}
}

func TestCreateDefaultsTypeToSystem(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := notifications.NewService(repo, &fakeChannels{}, &fakeDispatcher{})

	notification, err := svc.Create(context.Background(), orgID, createInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if notification.Type != notifications.TypeSystem {
		t.Fatalf("type = %q, want system", notification.Type)
	}

	if repo.created[0].Type != notifications.TypeSystem {
		t.Fatalf("persisted type = %q, want system", repo.created[0].Type)
	}
}

func TestCreatePersistsTypeAndLink(t *testing.T) {
	t.Parallel()

	repo := &fakeRepo{}
	svc := notifications.NewService(repo, &fakeChannels{}, &fakeDispatcher{})

	in := createInput()
	in.Type = notifications.TypeAssignment
	in.Link = "/assignments/asg-1"

	notification, err := svc.Create(context.Background(), orgID, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if notification.Type != notifications.TypeAssignment || notification.Link != "/assignments/asg-1" {
		t.Fatalf("type/link not stored: %+v", notification)
	}
}

func TestCreateRejectsExternalLink(t *testing.T) {
	t.Parallel()

	svc := notifications.NewService(&fakeRepo{}, &fakeChannels{}, &fakeDispatcher{})

	in := createInput()
	in.Link = "https://evil.example/phish"

	if _, err := svc.Create(context.Background(), orgID, in); !errors.Is(err, notifications.ErrInvalidInput) {
		t.Fatalf("got %v, want ErrInvalidInput", err)
	}
}

func TestCreateDeliversToConfiguredChannels(t *testing.T) {
	t.Parallel()

	channels := &fakeChannels{config: &notifications.ChannelConfig{
		OrganizationID:  orgID,
		SlackWebhookURL: "https://hooks.slack.com/services/T/B/X",
	}}
	dispatcher := &fakeDispatcher{}
	svc := notifications.NewService(&fakeRepo{}, channels, dispatcher)

	if _, err := svc.Create(context.Background(), orgID, createInput()); err != nil {
		t.Fatalf("create: %v", err)
	}

	if dispatcher.dispatched == nil {
		t.Fatalf("expected notification dispatched to channels")
	}

	if dispatcher.dispatched.SlackWebhookURL != "https://hooks.slack.com/services/T/B/X" {
		t.Fatalf("dispatched with wrong config: %+v", dispatcher.dispatched)
	}
}

func TestCreateSkipsDeliveryWhenUnconfigured(t *testing.T) {
	t.Parallel()

	dispatcher := &fakeDispatcher{}
	svc := notifications.NewService(&fakeRepo{}, &fakeChannels{}, dispatcher)

	notification, err := svc.Create(context.Background(), orgID, createInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if notification.Title != "Welcome" {
		t.Fatalf("unexpected notification: %+v", notification)
	}

	if dispatcher.dispatched != nil {
		t.Fatalf("expected no dispatch when channels are unconfigured")
	}
}

func TestFailedDeliveryPersistsAndManualRetryRecovers(t *testing.T) {
	t.Parallel()
	channels := &fakeChannels{config: &notifications.ChannelConfig{
		OrganizationID: orgID, SlackWebhookURL: "https://hooks.slack.com/services/T/B/X",
	}}
	dispatcher := &fakeDispatcher{err: errors.New("temporary outage")}
	deliveries := newFakeDeliveryStore()
	svc := notifications.NewService(&fakeRepo{}, channels, dispatcher).WithDeliveryStore(deliveries)
	if _, err := svc.Create(t.Context(), orgID, createInput()); err != nil {
		t.Fatalf("create: %v", err)
	}
	items, _ := svc.ListDeliveries(t.Context())
	if len(items) != 1 || items[0].Status != notifications.DeliveryRetrying || items[0].Attempts != 1 {
		t.Fatalf("unexpected failed delivery: %+v", items)
	}
	dispatcher.err = nil
	retried, err := svc.RetryDelivery(t.Context(), items[0].ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retried.Status != notifications.DeliveryDelivered || retried.Attempts != 2 {
		t.Fatalf("unexpected retried delivery: %+v", retried)
	}
}

func TestDeliveryMovesToDeadLetterAfterThreeFailures(t *testing.T) {
	t.Parallel()
	channels := &fakeChannels{config: &notifications.ChannelConfig{
		OrganizationID: orgID, SlackWebhookURL: "https://hooks.slack.com/services/T/B/X",
	}}
	dispatcher := &fakeDispatcher{err: errors.New("permanent outage")}
	deliveries := newFakeDeliveryStore()
	svc := notifications.NewService(&fakeRepo{}, channels, dispatcher).WithDeliveryStore(deliveries)
	if _, err := svc.Create(t.Context(), orgID, createInput()); err != nil {
		t.Fatalf("create: %v", err)
	}
	items, _ := svc.ListDeliveries(t.Context())
	for range 2 {
		if _, err := svc.RetryDelivery(t.Context(), items[0].ID); err != nil {
			t.Fatalf("retry: %v", err)
		}
	}
	got, _ := deliveries.GetDelivery(t.Context(), items[0].ID)
	if got.Status != notifications.DeliveryDeadLetter || got.Attempts != 3 || got.NextAttemptAt != nil {
		t.Fatalf("unexpected dead-letter delivery: %+v", got)
	}
}

func slackInput(rawURL string) notifications.SetChannelsInput {
	return notifications.SetChannelsInput{SlackWebhookURL: new(rawURL)}
}

func teamsInput(rawURL string) notifications.SetChannelsInput {
	return notifications.SetChannelsInput{TeamsWebhookURL: new(rawURL)}
}

func TestSetChannelConfigValidatesHosts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		in      notifications.SetChannelsInput
		wantErr bool
	}{
		{"valid slack", slackInput("https://hooks.slack.com/services/T/B/X"), false},
		{"valid teams office", teamsInput("https://x.webhook.office.com/webhookb2/a"), false},
		{"valid teams logic", teamsInput("https://prod-1.westus.logic.azure.com/workflows/a"), false},
		{"slack wrong host (SSRF)", slackInput("https://internal.corp.local/hook"), true},
		{"slack not https", slackInput("http://hooks.slack.com/x"), true},
		{"teams wrong host", teamsInput("https://hooks.slack.com/x"), true},
		{"clear slack", slackInput(""), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := notifications.NewService(&fakeRepo{}, &fakeChannels{}, &fakeDispatcher{})

			_, err := svc.SetChannelConfig(context.Background(), orgID, tc.in)
			if tc.wantErr && !errors.Is(err, notifications.ErrInvalidInput) {
				t.Fatalf("got %v, want ErrInvalidInput", err)
			}

			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetChannelConfigEmptyWhenUnset(t *testing.T) {
	t.Parallel()

	svc := notifications.NewService(&fakeRepo{}, &fakeChannels{}, &fakeDispatcher{})

	config, err := svc.GetChannelConfig(context.Background(), orgID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if config.OrganizationID != orgID || config.SlackWebhookURL != "" {
		t.Fatalf("expected empty config for org, got %+v", config)
	}
}

func (f *fakeRepo) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}

func (f *fakeChannels) DeleteForOrganization(context.Context, string) (int64, error) {
	return 0, nil
}
