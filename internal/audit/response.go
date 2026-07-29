package audit

import "time"

// EventResponse is the API representation of an Event. It decouples the public
// contract from the persistence layout.
type EventResponse struct {
	ID             string  `json:"id"`
	OrganizationID *string `json:"organizationId,omitempty"`
	ActorUserID    string  `json:"actorUserId"`
	ActorType      string  `json:"actorType"`
	Action         string  `json:"action"`
	ResourceType   string  `json:"resourceType"`
	ResourceID     string  `json:"resourceId"`
	IP             string  `json:"ip,omitempty"`
	UserAgent      string  `json:"userAgent,omitempty"`
	RequestID      string  `json:"requestId,omitempty"`
	Result         string  `json:"result,omitempty"`
	FailureReason  string  `json:"failureReason,omitempty"`
	Before         any     `json:"before,omitempty"`
	After          any     `json:"after,omitempty"`
	// ImpersonatorUserID and ImpersonationSessionID are present only when the
	// event was recorded under a platform support impersonation session.
	ImpersonatorUserID     string         `json:"impersonatorUserId,omitempty"`
	ImpersonationSessionID string         `json:"impersonationSessionId,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`
	CreatedAt              time.Time      `json:"createdAt"`
}

// ToResponse maps the persistence document to its API representation.
func (e Event) ToResponse() EventResponse {
	return EventResponse(e)
}

func toEventResponses(events []Event) []EventResponse {
	responses := make([]EventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, event.ToResponse())
	}

	return responses
}
