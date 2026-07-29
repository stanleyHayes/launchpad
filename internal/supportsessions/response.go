package supportsessions

import "time"

// SessionResponse is the API representation of a Session. It decouples the
// public contract from the persistence layout.
type SessionResponse struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	AgentUserID    string     `json:"agentUserId"`
	Reason         string     `json:"reason"`
	CreatedAt      time.Time  `json:"createdAt"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	EndedAt        *time.Time `json:"endedAt,omitempty"`
	EndReason      string     `json:"endReason,omitempty"`
}

// ToResponse maps the persistence document to its API representation.
func (s Session) ToResponse() SessionResponse {
	return SessionResponse(s)
}

// CreateSessionResponse returns the new session together with the
// impersonation token and its expiry. The token is shown exactly once, in
// this response; it is never persisted.
type CreateSessionResponse struct {
	Session        SessionResponse `json:"session"`
	Token          string          `json:"token"`
	TokenExpiresAt time.Time       `json:"tokenExpiresAt"`
}

func toSessionResponses(items []Session) []SessionResponse {
	responses := make([]SessionResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	return responses
}
