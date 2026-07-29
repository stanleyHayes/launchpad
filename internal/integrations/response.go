package integrations

import "time"

// ConnectionResponse is the API view of a Connection. It has no token field,
// so a stored credential can never be serialized to a client.
type ConnectionResponse struct {
	ID            string     `json:"id"`
	Provider      string     `json:"provider"`
	Status        string     `json:"status"`
	BaseURL       string     `json:"baseUrl,omitempty"`
	AccountHandle string     `json:"accountHandle"`
	LastSyncAt    *time.Time `json:"lastSyncAt,omitempty"`
	LastError     string     `json:"lastError,omitempty"`
	CreatedBy     string     `json:"createdBy"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// ToResponse maps a Connection to its API response DTO.
func (c Connection) ToResponse() ConnectionResponse {
	return ConnectionResponse{
		ID:            c.ID,
		Provider:      c.Provider,
		Status:        c.Status,
		BaseURL:       c.BaseURL,
		AccountHandle: c.AccountHandle,
		LastSyncAt:    c.LastSyncAt,
		LastError:     c.LastError,
		CreatedBy:     c.CreatedBy,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}

// ToResponses maps connections to their API response DTOs.
func ToResponses(connections []Connection) []ConnectionResponse {
	responses := make([]ConnectionResponse, 0, len(connections))
	for _, conn := range connections {
		responses = append(responses, conn.ToResponse())
	}

	return responses
}
