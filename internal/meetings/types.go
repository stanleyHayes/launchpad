// Package meetings manages org-scoped onboarding meetings (PRD §5.3.7):
// manager introductions, HR orientations, buddy check-ins, and the other
// scheduled touchpoints of a journey. Meetings work fully without a calendar
// connection (location is free text); when a tenant connects Google Calendar,
// an event is created best-effort and its reference stored on the meeting.
package meetings

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates the meeting or connection does not exist.
	ErrNotFound = errors.New("meeting not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid meeting input")
	// ErrInvalidState indicates an illegal status transition.
	ErrInvalidState = errors.New("invalid meeting state")
	// ErrMissingEventID indicates the provider returned an event without an id.
	ErrMissingEventID = errors.New("calendar provider returned an event without id")
	// ErrCalendarProvider indicates a non-credential provider failure.
	ErrCalendarProvider = errors.New("calendar provider request failed")
	// ErrForbidden indicates the actor may not access the meeting.
	ErrForbidden = errors.New("meeting access denied")
	// ErrInvalidCredential indicates the calendar provider rejected the token.
	ErrInvalidCredential = errors.New("calendar provider rejected the credential")
)

// Meeting types (PRD §5.3.7).
const (
	TypeManagerIntro            = "manager_intro"
	TypeHROrientation           = "hr_orientation"
	TypeTeamIntro               = "team_intro"
	TypeBuddyCheckin            = "buddy_checkin"
	TypeArchitectureWalkthrough = "architecture_walkthrough"
	TypeRoleCoaching            = "role_coaching"
	TypeFirstWeekReview         = "first_week_review"
)

const (
	statusScheduled = "scheduled"
	statusCompleted = "completed"
	statusCancelled = "cancelled"
	statusNoShow    = "no_show"
)

const (
	ProviderGoogle    = "google"
	ProviderMicrosoft = "microsoft"
)

const (
	minDurationMin = 5
	maxDurationMin = 480
)

// Meeting is one org-scoped scheduled touchpoint between an organizer and an
// employee.
type Meeting struct {
	ID                 string     `bson:"_id"                        json:"id"`
	OrganizationID     string     `bson:"organizationId"             json:"organizationId"`
	Title              string     `bson:"title"                      json:"title"`
	Type               string     `bson:"type"                       json:"type"`
	OrganizerUserID    string     `bson:"organizerUserId,omitempty"  json:"organizerUserId,omitempty"`
	AttendeeEmployeeID string     `bson:"attendeeEmployeeId"         json:"attendeeEmployeeId"`
	StartsAt           time.Time  `bson:"startsAt"                   json:"startsAt"`
	DurationMin        int        `bson:"durationMin"                json:"durationMin"`
	Location           string     `bson:"location,omitempty"         json:"location,omitempty"`
	Status             string     `bson:"status"                     json:"status"`
	NotesLink          string     `bson:"notesLink,omitempty"        json:"notesLink,omitempty"`
	CalendarEventRef   string     `bson:"calendarEventRef,omitempty" json:"calendarEventRef,omitempty"`
	CreatedAt          time.Time  `bson:"createdAt"                  json:"createdAt"`
	UpdatedAt          time.Time  `bson:"updatedAt"                  json:"updatedAt"`
	CompletedAt        *time.Time `bson:"completedAt,omitempty"      json:"completedAt,omitempty"`
	ReminderNotifiedAt *time.Time `bson:"reminderNotifiedAt,omitempty" json:"reminderNotifiedAt,omitempty"`
}

// CalendarConnection is a tenant's link to one calendar provider. Token only
// travels between service and store: the Mongo adapter encrypts it at rest,
// and API responses use CalendarConnectionResponse, which has no token field.
type CalendarConnection struct {
	ID             string     `bson:"_id"                       json:"id"`
	OrganizationID string     `bson:"organizationId"            json:"organizationId"`
	Provider       string     `bson:"provider"                  json:"provider"`
	AccountHandle  string     `bson:"accountHandle"             json:"accountHandle"`
	Token          string     `bson:"calendarToken"             json:"-"`
	RefreshToken   string     `bson:"calendarRefreshToken,omitempty" json:"-"`
	TokenExpiresAt *time.Time `bson:"tokenExpiresAt,omitempty"  json:"-"`
	LastSyncAt     *time.Time `bson:"lastSyncAt,omitempty"      json:"lastSyncAt,omitempty"`
	LastError      string     `bson:"lastError,omitempty"       json:"lastError,omitempty"`
	CreatedBy      string     `bson:"createdBy"                 json:"createdBy"`
	CreatedAt      time.Time  `bson:"createdAt"                 json:"createdAt"`
	UpdatedAt      time.Time  `bson:"updatedAt"                 json:"updatedAt"`
}

// CreateInput schedules a meeting for an employee.
type CreateInput struct {
	OrganizationID     string
	Title              string
	Type               string
	OrganizerUserID    string
	AttendeeEmployeeID string
	StartsAt           time.Time
	DurationMin        int
	Location           string
	NotesLink          string
}

// CompleteInput records the outcome of a scheduled meeting.
type CompleteInput struct {
	OrganizationID string
	MeetingID      string
	// NoShow marks the meeting as missed instead of completed.
	NoShow    bool
	NotesLink string
}

// RescheduleInput changes the timing and location of a scheduled meeting.
type RescheduleInput struct {
	OrganizationID string
	MeetingID      string
	StartsAt       time.Time
	DurationMin    int
	Location       string
}

func isValidType(meetingType string) bool {
	switch meetingType {
	case TypeManagerIntro, TypeHROrientation, TypeTeamIntro, TypeBuddyCheckin,
		TypeArchitectureWalkthrough, TypeRoleCoaching, TypeFirstWeekReview:
		return true
	default:
		return false
	}
}

func isValidStatus(status string) bool {
	switch status {
	case statusScheduled, statusCompleted, statusCancelled, statusNoShow:
		return true
	default:
		return false
	}
}
