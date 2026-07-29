package roles

import "time"

// RoleResponse is the API representation of a Role. It decouples the public
// contract from the persistence layout.
type RoleResponse struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	Permissions    []string  `json:"permissions"`
	Builtin        bool      `json:"builtin"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// ToResponse maps the persistence document to its API representation.
func (r Role) ToResponse() RoleResponse {
	return RoleResponse(r)
}

// ToResponses maps a slice of roles to their API representations.
func ToResponses(items []Role) []RoleResponse {
	out := make([]RoleResponse, 0, len(items))
	for _, item := range items {
		out = append(out, item.ToResponse())
	}

	return out
}
