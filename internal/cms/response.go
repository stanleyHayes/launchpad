package cms

import "time"

// PageResponse is the API representation of a Page. It decouples the public
// contract from the persistence layout.
type PageResponse struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Summary     string     `json:"summary"`
	Body        string     `json:"body"`
	ContentType string     `json:"contentType"`
	NavLabel    string     `json:"navLabel,omitempty"`
	NavOrder    int        `json:"navOrder,omitempty"`
	Status      string     `json:"status"`
	ScheduledAt *time.Time `json:"scheduledAt,omitempty"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// ToResponse maps the persistence document to its API representation.
func (p Page) ToResponse() PageResponse {
	return PageResponse{
		ID: p.ID, Slug: p.Slug, Title: p.Title, Summary: p.Summary, Body: p.Body,
		ContentType: p.ContentType, NavLabel: p.NavLabel, NavOrder: p.NavOrder,
		Status: p.Status, ScheduledAt: p.ScheduledAt, PublishedAt: p.PublishedAt,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func toPageResponses(items []Page) []PageResponse {
	responses := make([]PageResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	return responses
}
