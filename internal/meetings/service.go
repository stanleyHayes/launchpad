package meetings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/internal/employees"
)

const (
	defaultDurationMin = 30
	// teamListLimit bounds the employee page scanned to resolve a manager's
	// team when scoping the meetings list.
	teamListLimit = 1000
)

// EmployeeDirectory resolves employees for self-service and team scoping.
// Implemented by internal/employees's service.
type EmployeeDirectory interface {
	GetByUserID(ctx context.Context, organizationID, userID string) (employees.Employee, error)
	Get(ctx context.Context, organizationID, employeeID string) (employees.Employee, error)
	List(ctx context.Context, organizationID string, offset, limit int64) ([]employees.Employee, error)
}

// Service implements meeting use cases. The calendar connection repository,
// employee directory, and calendar client are all nil-safe: meetings work
// fully without them (self-service endpoints report ErrInvalidState without
// an employee directory; calendar sync is skipped without a connection or
// client).
type Service struct {
	repo              Repository
	connections       ConnectionRepository
	employees         EmployeeDirectory
	calendar          CalendarClient
	microsoftCalendar CalendarClient
	oauth             map[string]*OAuthClient
}

// WithOAuthClients enables authorization-code connection and automatic access
// token refresh for configured providers.
func (s *Service) WithOAuthClients(google, microsoft *OAuthClient) *Service {
	s.oauth = map[string]*OAuthClient{
		ProviderGoogle: google, ProviderMicrosoft: microsoft,
	}

	return s
}

// WithMicrosoftCalendar adds Microsoft Graph as a second calendar provider.
func (s *Service) WithMicrosoftCalendar(client CalendarClient) *Service {
	s.microsoftCalendar = client
	return s
}

// NewService constructs a Service.
func NewService(
	repo Repository,
	connections ConnectionRepository,
	employeeDirectory EmployeeDirectory,
	calendar CalendarClient,
) *Service {
	return &Service{
		repo:        repo,
		connections: connections,
		employees:   employeeDirectory,
		calendar:    calendar,
	}
}

// Create schedules a meeting. When the tenant has a calendar connection and
// a calendar client is wired, a provider event is created best-effort and its
// reference stored; a provider failure never blocks the meeting.
func (s *Service) Create(ctx context.Context, in CreateInput) (Meeting, error) {
	title := strings.TrimSpace(in.Title)

	if in.OrganizationID == "" || in.AttendeeEmployeeID == "" || title == "" ||
		!isValidType(in.Type) || in.StartsAt.IsZero() {
		return Meeting{}, ErrInvalidInput
	}

	duration := in.DurationMin
	if duration == 0 {
		duration = defaultDurationMin
	}

	if duration < minDurationMin || duration > maxDurationMin {
		return Meeting{}, ErrInvalidInput
	}

	now := time.Now().UTC()

	meeting := Meeting{
		ID:                 uuid.NewString(),
		OrganizationID:     in.OrganizationID,
		Title:              title,
		Type:               in.Type,
		OrganizerUserID:    in.OrganizerUserID,
		AttendeeEmployeeID: in.AttendeeEmployeeID,
		StartsAt:           in.StartsAt.UTC(),
		DurationMin:        duration,
		Location:           strings.TrimSpace(in.Location),
		Status:             statusScheduled,
		NotesLink:          strings.TrimSpace(in.NotesLink),
		CalendarEventRef:   "",
		CreatedAt:          now,
		UpdatedAt:          now,
		CompletedAt:        nil,
	}

	meeting.CalendarEventRef = s.syncCalendarEvent(ctx, meeting)

	if err := s.repo.Create(ctx, meeting); err != nil {
		return Meeting{}, fmt.Errorf("create meeting: %w", err)
	}

	return meeting, nil
}

// CreateFromStep schedules the meeting backing a meeting journey step when
// the employee submits the schedule form (assignments.MeetingScheduler port).
// An empty meeting type defaults to manager_intro; startsAt must be an
// RFC3339 timestamp — without a scheduled time there is no meeting and the
// step must not complete.
func (s *Service) CreateFromStep(
	ctx context.Context,
	organizationID, employeeID, meetingType, title, startsAt string,
	durationMin int,
	location string,
) error {
	if meetingType == "" {
		meetingType = TypeManagerIntro
	}

	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(startsAt))
	if err != nil {
		return fmt.Errorf("parse meeting start: %w", ErrInvalidInput)
	}

	_, err = s.Create(ctx, CreateInput{
		OrganizationID:     organizationID,
		Title:              title,
		Type:               meetingType,
		OrganizerUserID:    s.resolveManagerUserID(ctx, organizationID, employeeID),
		AttendeeEmployeeID: employeeID,
		StartsAt:           parsed,
		DurationMin:        durationMin,
		Location:           location,
		NotesLink:          "",
	})

	return err
}

// ListMine returns the caller's own meetings, soonest first.
func (s *Service) ListMine(ctx context.Context, organizationID, userID string) ([]Meeting, error) {
	employee, err := s.resolveEmployee(ctx, organizationID, userID)
	if err != nil {
		return nil, err
	}

	items, err := s.repo.ListByAttendee(ctx, organizationID, employee.ID)
	if err != nil {
		return nil, fmt.Errorf("list meetings: %w", err)
	}

	return items, nil
}

// CancelMine cancels one of the caller's own scheduled meetings.
func (s *Service) CancelMine(ctx context.Context, organizationID, userID, meetingID string) (Meeting, error) {
	employee, err := s.resolveEmployee(ctx, organizationID, userID)
	if err != nil {
		return Meeting{}, err
	}

	meeting, err := s.repo.GetByIDForOrganization(ctx, organizationID, meetingID)
	if err != nil {
		return Meeting{}, fmt.Errorf("get meeting: %w", err)
	}

	if meeting.AttendeeEmployeeID != employee.ID {
		return Meeting{}, ErrForbidden
	}

	return s.transition(ctx, meeting, statusCancelled, "", nil)
}

// ListForUser returns meetings visible to a manager or admin caller. A caller
// with an employee record sees their team's meetings (direct reports plus
// meetings they organize or attend); a caller without one (e.g. an HR admin
// with no employee profile) sees the whole organization. Route-level
// permission (employees.read) remains the authorization gate.
func (s *Service) ListForUser(ctx context.Context, organizationID, userID, status string) ([]Meeting, error) {
	status = strings.TrimSpace(status)
	if status != "" && !isValidStatus(status) {
		return nil, ErrInvalidInput
	}

	items, err := s.repo.ListByOrganization(ctx, organizationID, status)
	if err != nil {
		return nil, fmt.Errorf("list meetings: %w", err)
	}

	visible, err := s.visibleAttendees(ctx, organizationID, userID)
	if err != nil {
		return nil, err
	}

	// A caller with no employee record (e.g. an HR admin) sees the whole
	// organization. visibleAttendees uses nil to distinguish that case from
	// an employee with no direct reports.
	if visible == nil {
		return items, nil
	}

	scoped := make([]Meeting, 0, len(items))
	for _, item := range items {
		if visible[item.AttendeeEmployeeID] || item.OrganizerUserID == userID {
			scoped = append(scoped, item)
		}
	}

	return scoped, nil
}

// Complete records the outcome of a scheduled meeting (held or no-show, with
// an optional notes link — PRD §5.3.7 attendance tracking and notes links).
func (s *Service) Complete(ctx context.Context, in CompleteInput) (Meeting, error) {
	if in.OrganizationID == "" || in.MeetingID == "" {
		return Meeting{}, ErrInvalidInput
	}

	meeting, err := s.repo.GetByIDForOrganization(ctx, in.OrganizationID, in.MeetingID)
	if err != nil {
		return Meeting{}, fmt.Errorf("get meeting: %w", err)
	}

	outcome := statusCompleted
	if in.NoShow {
		outcome = statusNoShow
	}

	now := time.Now().UTC()

	return s.transition(ctx, meeting, outcome, in.NotesLink, &now)
}

// Cancel cancels a scheduled meeting (manager/admin action).
func (s *Service) Cancel(ctx context.Context, organizationID, meetingID string) (Meeting, error) {
	if organizationID == "" || meetingID == "" {
		return Meeting{}, ErrInvalidInput
	}

	meeting, err := s.repo.GetByIDForOrganization(ctx, organizationID, meetingID)
	if err != nil {
		return Meeting{}, fmt.Errorf("get meeting: %w", err)
	}

	return s.transition(ctx, meeting, statusCancelled, "", nil)
}

// GetCalendarConnection returns the tenant's Google Calendar connection as a
// masked DTO (no token), or ErrNotFound when none is connected.
func (s *Service) GetCalendarConnection(
	ctx context.Context,
	organizationID string,
) (CalendarConnectionResponse, error) {
	if organizationID == "" {
		return CalendarConnectionResponse{}, ErrInvalidInput
	}

	conn, err := s.getConnection(ctx, organizationID)
	if err != nil {
		return CalendarConnectionResponse{}, err
	}

	return conn.ToResponse(), nil
}

func (s *Service) GetCalendarConnectionForProvider(
	ctx context.Context,
	organizationID, provider string,
) (CalendarConnectionResponse, error) {
	if organizationID == "" || !validProvider(provider) {
		return CalendarConnectionResponse{}, ErrInvalidInput
	}
	if s.connections == nil {
		return CalendarConnectionResponse{}, ErrNotFound
	}
	conn, err := s.connections.Get(ctx, organizationID, provider)
	if err != nil {
		return CalendarConnectionResponse{}, err
	}
	return conn.ToResponse(), nil
}

// ConnectCalendar validates a Google Calendar access token BEFORE persisting
// anything, then upserts the tenant's connection. Manual token entry is the
// interim flow until OAuth app registrations exist (stop rule); reconnecting
// replaces the connection (idempotent upsert).
func (s *Service) ConnectCalendar(
	ctx context.Context,
	organizationID, actorUserID, token string,
) (CalendarConnectionResponse, error) {
	return s.ConnectCalendarProvider(ctx, organizationID, actorUserID, ProviderGoogle, token)
}

func (s *Service) ConnectCalendarProvider(
	ctx context.Context,
	organizationID, actorUserID, provider, token string,
) (CalendarConnectionResponse, error) {
	return s.connectCalendarCredential(ctx, organizationID, actorUserID, provider, OAuthToken{AccessToken: token})
}

func (s *Service) connectCalendarCredential(
	ctx context.Context,
	organizationID, actorUserID, provider string,
	credential OAuthToken,
) (CalendarConnectionResponse, error) {
	token := credential.AccessToken
	if organizationID == "" || strings.TrimSpace(token) == "" {
		return CalendarConnectionResponse{}, ErrInvalidInput
	}

	client := s.calendarForProvider(provider)
	if client == nil || s.connections == nil {
		return CalendarConnectionResponse{}, ErrInvalidState
	}

	handle, err := client.Validate(ctx, strings.TrimSpace(token))
	if err != nil {
		return CalendarConnectionResponse{}, fmt.Errorf("validate calendar credential: %w", err)
	}

	now := time.Now().UTC()
	conn := CalendarConnection{
		ID:             uuid.NewString(),
		OrganizationID: organizationID,
		Provider:       provider,
		AccountHandle:  handle,
		Token:          strings.TrimSpace(token),
		RefreshToken:   strings.TrimSpace(credential.RefreshToken),
		TokenExpiresAt: credential.ExpiresAt,
		LastSyncAt:     &now,
		LastError:      "",
		CreatedBy:      actorUserID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	existing, err := s.connections.Get(ctx, organizationID, provider)
	switch {
	case err == nil:
		conn.ID = existing.ID
		conn.CreatedBy = existing.CreatedBy
		conn.CreatedAt = existing.CreatedAt
	case errors.Is(err, ErrNotFound):
	default:
		return CalendarConnectionResponse{}, fmt.Errorf("load existing calendar connection: %w", err)
	}

	if err := s.connections.Upsert(ctx, conn); err != nil {
		return CalendarConnectionResponse{}, fmt.Errorf("upsert calendar connection: %w", err)
	}

	return conn.ToResponse(), nil
}

// ConnectCalendarOAuth validates and persists a provider-issued OAuth token
// pair after the authorization-code callback.
func (s *Service) ConnectCalendarOAuth(
	ctx context.Context,
	organizationID, actorUserID, provider string,
	credential OAuthToken,
) (CalendarConnectionResponse, error) {
	return s.connectCalendarCredential(ctx, organizationID, actorUserID, provider, credential)
}

// DisconnectCalendar removes the tenant's Google Calendar connection.
func (s *Service) DisconnectCalendar(ctx context.Context, organizationID string) error {
	return s.DisconnectCalendarProvider(ctx, organizationID, ProviderGoogle)
}

func (s *Service) DisconnectCalendarProvider(ctx context.Context, organizationID, provider string) error {
	if organizationID == "" {
		return ErrInvalidInput
	}

	if s.connections == nil {
		return ErrInvalidState
	}

	if !validProvider(provider) {
		return ErrInvalidInput
	}
	if err := s.connections.Delete(ctx, organizationID, provider); err != nil {
		return fmt.Errorf("delete calendar connection: %w", err)
	}

	return nil
}

// Reschedule changes a scheduled meeting and updates its provider event
// best-effort. A new reminder may be emitted for the changed time.
func (s *Service) Reschedule(ctx context.Context, in RescheduleInput) (Meeting, error) {
	if in.OrganizationID == "" || in.MeetingID == "" || in.StartsAt.IsZero() ||
		in.DurationMin < minDurationMin || in.DurationMin > maxDurationMin {
		return Meeting{}, ErrInvalidInput
	}
	meeting, err := s.repo.GetByIDForOrganization(ctx, in.OrganizationID, in.MeetingID)
	if err != nil {
		return Meeting{}, err
	}
	if meeting.Status != statusScheduled {
		return Meeting{}, ErrInvalidState
	}
	meeting.StartsAt = in.StartsAt.UTC()
	meeting.DurationMin = in.DurationMin
	meeting.Location = strings.TrimSpace(in.Location)
	meeting.ReminderNotifiedAt = nil
	meeting.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, meeting); err != nil {
		return Meeting{}, fmt.Errorf("reschedule meeting: %w", err)
	}
	s.updateCalendarEvent(ctx, meeting)

	return meeting, nil
}

// transition moves a scheduled meeting into a terminal status.
func (s *Service) transition(
	ctx context.Context,
	meeting Meeting,
	status, notesLink string,
	completedAt *time.Time,
) (Meeting, error) {
	if meeting.Status != statusScheduled {
		return Meeting{}, ErrInvalidState
	}

	meeting.Status = status
	if trimmed := strings.TrimSpace(notesLink); trimmed != "" {
		meeting.NotesLink = trimmed
	}

	meeting.CompletedAt = completedAt
	meeting.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, meeting); err != nil {
		return Meeting{}, fmt.Errorf("update meeting: %w", err)
	}

	return meeting, nil
}

// syncCalendarEvent creates a provider event for the meeting when a calendar
// connection and client are available. It is best-effort: any failure is
// logged and the meeting proceeds without a calendar reference.
func (s *Service) syncCalendarEvent(ctx context.Context, meeting Meeting) string {
	if s.connections == nil {
		return ""
	}
	for _, provider := range []string{ProviderGoogle, ProviderMicrosoft} {
		client := s.calendarForProvider(provider)
		if client == nil {
			continue
		}
		conn, err := s.connections.Get(ctx, meeting.OrganizationID, provider)
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				slog.ErrorContext(ctx, "load calendar connection for meeting sync", "provider", provider, "error", err)
			}
			continue
		}
		conn, err = s.refreshConnectionIfNeeded(ctx, conn)
		if err != nil {
			slog.ErrorContext(ctx, "refresh calendar access token", "provider", provider, "error", err)
			continue
		}
		ref, createErr := client.CreateEvent(ctx, conn.Token, CalendarEvent{
			Title: meeting.Title, Location: meeting.Location, StartsAt: meeting.StartsAt, DurationMin: meeting.DurationMin,
		})
		if createErr != nil {
			slog.ErrorContext(ctx, "calendar event sync failed", "provider", provider, "meetingId", meeting.ID, "error", createErr)
			continue
		}
		if provider == ProviderMicrosoft {
			return provider + ":" + ref
		}
		return ref
	}
	return ""
}

func (s *Service) updateCalendarEvent(ctx context.Context, meeting Meeting) {
	if meeting.CalendarEventRef == "" || s.connections == nil {
		return
	}
	provider, eventID := ProviderGoogle, meeting.CalendarEventRef
	if strings.HasPrefix(eventID, ProviderMicrosoft+":") {
		provider, eventID = ProviderMicrosoft, strings.TrimPrefix(eventID, ProviderMicrosoft+":")
	}
	updater, ok := s.calendarForProvider(provider).(CalendarEventUpdater)
	if !ok {
		return
	}
	conn, err := s.connections.Get(ctx, meeting.OrganizationID, provider)
	if err != nil {
		return
	}
	conn, err = s.refreshConnectionIfNeeded(ctx, conn)
	if err != nil {
		slog.ErrorContext(ctx, "refresh calendar token for reschedule", "provider", provider, "error", err)
		return
	}
	err = updater.UpdateEvent(ctx, conn.Token, eventID, CalendarEvent{
		Title: meeting.Title, Location: meeting.Location, StartsAt: meeting.StartsAt, DurationMin: meeting.DurationMin,
	})
	if err != nil {
		slog.ErrorContext(ctx, "update calendar event after reschedule", "provider", provider, "meetingId", meeting.ID, "error", err)
	}
}

func (s *Service) refreshConnectionIfNeeded(
	ctx context.Context,
	conn CalendarConnection,
) (CalendarConnection, error) {
	if conn.TokenExpiresAt == nil || time.Now().UTC().Add(time.Minute).Before(*conn.TokenExpiresAt) {
		return conn, nil
	}
	oauthClient := s.oauth[conn.Provider]
	if oauthClient == nil || conn.RefreshToken == "" {
		return CalendarConnection{}, ErrInvalidCredential
	}
	refreshed, err := oauthClient.Refresh(ctx, conn.RefreshToken)
	if err != nil {
		return CalendarConnection{}, err
	}
	conn.Token = refreshed.AccessToken
	conn.RefreshToken = refreshed.RefreshToken
	conn.TokenExpiresAt = refreshed.ExpiresAt
	conn.UpdatedAt = time.Now().UTC()
	if err := s.connections.Upsert(ctx, conn); err != nil {
		return CalendarConnection{}, err
	}

	return conn, nil
}

// resolveManagerUserID best-effort resolves the employee's manager's user id
// to organize a step-created meeting. Empty when unresolvable.
func (s *Service) resolveManagerUserID(ctx context.Context, organizationID, employeeID string) string {
	if s.employees == nil {
		return ""
	}

	employee, err := s.employees.Get(ctx, organizationID, employeeID)
	if err != nil || employee.ManagerEmployeeID == "" {
		return ""
	}

	manager, err := s.employees.Get(ctx, organizationID, employee.ManagerEmployeeID)
	if err != nil {
		return ""
	}

	return manager.UserID
}

// visibleAttendees returns the attendee ids a caller may see: nil for a
// caller with no employee record (org-wide visibility), otherwise the
// caller's own id plus their direct reports'.
func (s *Service) visibleAttendees(
	ctx context.Context,
	organizationID, userID string,
) (map[string]bool, error) {
	if s.employees == nil {
		return nil, nil
	}

	caller, err := s.employees.GetByUserID(ctx, organizationID, userID)
	if errors.Is(err, employees.ErrNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("resolve caller employee: %w", err)
	}

	visible := map[string]bool{caller.ID: true}

	team, err := s.employees.List(ctx, organizationID, 0, teamListLimit)
	if err != nil {
		return nil, fmt.Errorf("list team employees: %w", err)
	}

	for _, member := range team {
		if member.ManagerEmployeeID == caller.ID {
			visible[member.ID] = true
		}
	}

	return visible, nil
}

func (s *Service) resolveEmployee(
	ctx context.Context,
	organizationID, userID string,
) (employees.Employee, error) {
	if s.employees == nil {
		return employees.Employee{}, ErrInvalidState
	}

	employee, err := s.employees.GetByUserID(ctx, organizationID, userID)
	if err != nil {
		if errors.Is(err, employees.ErrNotFound) {
			return employees.Employee{}, ErrForbidden
		}

		return employees.Employee{}, fmt.Errorf("resolve employee: %w", err)
	}

	return employee, nil
}

func (s *Service) getConnection(ctx context.Context, organizationID string) (CalendarConnection, error) {
	if s.connections == nil {
		return CalendarConnection{}, ErrNotFound
	}

	conn, err := s.connections.Get(ctx, organizationID, ProviderGoogle)
	if err != nil {
		return CalendarConnection{}, err
	}

	return conn, nil
}

func validProvider(provider string) bool {
	return provider == ProviderGoogle || provider == ProviderMicrosoft
}

func (s *Service) calendarForProvider(provider string) CalendarClient {
	switch provider {
	case ProviderGoogle:
		return s.calendar
	case ProviderMicrosoft:
		return s.microsoftCalendar
	default:
		return nil
	}
}
