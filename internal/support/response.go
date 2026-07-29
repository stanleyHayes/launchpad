package support

import "time"

// TicketResponse is the API representation of a Ticket. It decouples the
// public contract from the persistence layout.
type TicketResponse struct {
	ID              string          `json:"id"`
	OrganizationID  string          `json:"organizationId"`
	CreatedByUserID string          `json:"createdByUserId"`
	Subject         string          `json:"subject"`
	Body            string          `json:"body"`
	Priority        string          `json:"priority"`
	Category        string          `json:"category,omitempty"`
	Status          string          `json:"status"`
	AssigneeUserID  string          `json:"assigneeUserId,omitempty"`
	SLADueAt        time.Time       `json:"slaDueAt"`
	FirstResponseAt *time.Time      `json:"firstResponseAt,omitempty"`
	ResolvedAt      *time.Time      `json:"resolvedAt,omitempty"`
	EscalationCount int             `json:"escalationCount"`
	Tags            []string        `json:"tags,omitempty"`
	Messages        []TicketMessage `json:"messages,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// ToResponse maps the persistence document to its API representation.
func (t Ticket) ToResponse() TicketResponse {
	return TicketResponse(t)
}

func toTicketResponses(items []Ticket) []TicketResponse {
	responses := make([]TicketResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	return responses
}

// BlockerResponse is the API representation of a Blocker.
type BlockerResponse struct {
	ID               string    `json:"id"`
	OrganizationID   string    `json:"organizationId"`
	EmployeeID       string    `json:"employeeId"`
	ReportedByUserID string    `json:"reportedByUserId"`
	StepAssignmentID string    `json:"stepAssignmentId,omitempty"`
	Category         string    `json:"category"`
	Message          string    `json:"message"`
	TicketID         string    `json:"ticketId"`
	CreatedAt        time.Time `json:"createdAt"`
}

// ToResponse maps the persistence document to its API representation.
func (b Blocker) ToResponse() BlockerResponse {
	return BlockerResponse(b)
}

// ToBlockerResponses maps blockers to their API representations. Exported so
// the assignments module can return blockers on manager-facing endpoints.
func ToBlockerResponses(items []Blocker) []BlockerResponse {
	responses := make([]BlockerResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	return responses
}
