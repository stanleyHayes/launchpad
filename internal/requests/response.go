package requests

import "time"

// Response is the API representation of a Request. It decouples the public
// contract from the persistence layout.
type Response struct {
	ID                  string     `json:"id"`
	OrganizationID      string     `json:"organizationId"`
	Kind                string     `json:"kind"`
	Item                string     `json:"item"`
	Details             string     `json:"details,omitempty"`
	Status              string     `json:"status"`
	RequesterEmployeeID string     `json:"requesterEmployeeId"`
	ApproverUserID      string     `json:"approverUserId,omitempty"`
	DecisionNote        string     `json:"decisionNote,omitempty"`
	DecidedAt           *time.Time `json:"decidedAt,omitempty"`
	FulfilledAt         *time.Time `json:"fulfilledAt,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

// ToResponse maps the persistence document to its API representation.
func (r Request) ToResponse() Response {
	return Response(r)
}

func toResponses(items []Request) []Response {
	responses := make([]Response, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	return responses
}
