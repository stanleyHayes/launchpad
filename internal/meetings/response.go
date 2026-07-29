package meetings

import "time"

// Response is the API representation of a Meeting. It decouples the public
// contract from the persistence layout.
type Response struct {
	ID                 string     `json:"id"`
	OrganizationID     string     `json:"organizationId"`
	Title              string     `json:"title"`
	Type               string     `json:"type"`
	OrganizerUserID    string     `json:"organizerUserId,omitempty"`
	AttendeeEmployeeID string     `json:"attendeeEmployeeId"`
	StartsAt           time.Time  `json:"startsAt"`
	DurationMin        int        `json:"durationMin"`
	Location           string     `json:"location,omitempty"`
	Status             string     `json:"status"`
	NotesLink          string     `json:"notesLink,omitempty"`
	CalendarEventRef   string     `json:"calendarEventRef,omitempty"`
	CompletedAt        *time.Time `json:"completedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// ToResponse maps the persistence document to its API representation.
func (m Meeting) ToResponse() Response {
	return Response{
		ID:                 m.ID,
		OrganizationID:     m.OrganizationID,
		Title:              m.Title,
		Type:               m.Type,
		OrganizerUserID:    m.OrganizerUserID,
		AttendeeEmployeeID: m.AttendeeEmployeeID,
		StartsAt:           m.StartsAt,
		DurationMin:        m.DurationMin,
		Location:           m.Location,
		Status:             m.Status,
		NotesLink:          m.NotesLink,
		CalendarEventRef:   m.CalendarEventRef,
		CompletedAt:        m.CompletedAt,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
	}
}

func toResponses(items []Meeting) []Response {
	responses := make([]Response, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	return responses
}

// CalendarConnectionResponse is the API view of a CalendarConnection. It has
// no token field, so a stored credential can never be serialized to a client.
type CalendarConnectionResponse struct {
	ID            string     `json:"id"`
	Provider      string     `json:"provider"`
	AccountHandle string     `json:"accountHandle"`
	Connected     bool       `json:"connected"`
	LastSyncAt    *time.Time `json:"lastSyncAt,omitempty"`
	LastError     string     `json:"lastError,omitempty"`
	CreatedBy     string     `json:"createdBy"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// ToResponse maps a CalendarConnection to its secret-free API representation.
func (c CalendarConnection) ToResponse() CalendarConnectionResponse {
	return CalendarConnectionResponse{
		ID:            c.ID,
		Provider:      c.Provider,
		AccountHandle: c.AccountHandle,
		Connected:     true,
		LastSyncAt:    c.LastSyncAt,
		LastError:     c.LastError,
		CreatedBy:     c.CreatedBy,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}
