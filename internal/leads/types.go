package leads

import (
	"errors"
	"time"
)

var (
	// ErrNotFound indicates the lead does not exist.
	ErrNotFound = errors.New("lead not found")
	// ErrInvalidInput indicates validation failed.
	ErrInvalidInput = errors.New("invalid lead input")
)

const (
	statusNew       = "new"
	statusContacted = "contacted"
	statusQualified = "qualified"
	statusClosed    = "closed"
	defaultSource   = "website"
)

// Lead is a marketing or sales inquiry.
type Lead struct {
	ID        string    `bson:"_id"       json:"id"`
	Name      string    `bson:"name"      json:"name"`
	Email     string    `bson:"email"     json:"email"`
	Company   string    `bson:"company"   json:"company"`
	Message   string    `bson:"message"   json:"message"`
	Source    string    `bson:"source"    json:"source"`
	Status    string    `bson:"status"    json:"status"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
}

// CreateInput is the public lead capture payload.
type CreateInput struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Company string `json:"company"`
	Message string `json:"message"`
	Source  string `json:"source"`
}
