package organizations

import "time"

// OrganizationResponse is the API representation of an Organization. It
// decouples the public contract from the persistence layout.
type OrganizationResponse struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Slug             string           `json:"slug"`
	Status           string           `json:"status"`
	PlanCode         string           `json:"planCode"`
	Timezone         string           `json:"timezone"`
	CustomDomain     string           `json:"customDomain,omitempty"`
	Branding         BrandingResponse `json:"branding"`
	SetupStep        int              `json:"setupStep"`
	SetupCompletedAt *time.Time       `json:"setupCompletedAt,omitempty"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

// BrandingResponse is the API representation of an organization's Branding.
type BrandingResponse struct {
	PrimaryColor      string `json:"primaryColor,omitempty"`
	PrimaryHoverColor string `json:"primaryHoverColor,omitempty"`
	AccentColor       string `json:"accentColor,omitempty"`
	LogoURL           string `json:"logoUrl,omitempty"`
}

// ToResponse maps the persistence document to its API representation.
func (o Organization) ToResponse() OrganizationResponse {
	return OrganizationResponse{
		ID:               o.ID,
		Name:             o.Name,
		Slug:             o.Slug,
		Status:           o.Status,
		PlanCode:         o.PlanCode,
		Timezone:         o.Timezone,
		CustomDomain:     o.CustomDomain,
		Branding:         o.Branding.ToResponse(),
		SetupStep:        o.SetupStep,
		SetupCompletedAt: o.SetupCompletedAt,
		CreatedAt:        o.CreatedAt,
		UpdatedAt:        o.UpdatedAt,
	}
}

// ToResponse maps the persistence sub-document to its API representation.
func (b Branding) ToResponse() BrandingResponse {
	return BrandingResponse(b)
}

// MemberResponse is the API representation of a Member: the membership plus
// the account's display info. It keeps the persistence documents out of the
// API contract.
type MemberResponse struct {
	UserID       string    `json:"userId"`
	RoleCode     string    `json:"roleCode"`
	Status       string    `json:"status"`
	Email        string    `json:"email,omitempty"`
	DisplayName  string    `json:"displayName,omitempty"`
	UserStatus   string    `json:"userStatus,omitempty"`
	MembershipID string    `json:"membershipId"`
	CreatedAt    time.Time `json:"createdAt"`
}

// ToResponse maps a Member to its API representation.
func (m Member) ToResponse() MemberResponse {
	return MemberResponse{
		UserID:       m.Membership.UserID,
		RoleCode:     m.Membership.RoleCode,
		Status:       m.Membership.Status,
		Email:        m.Email,
		DisplayName:  m.DisplayName,
		UserStatus:   m.UserStatus,
		MembershipID: m.Membership.ID,
		CreatedAt:    m.Membership.CreatedAt,
	}
}

// ToMemberResponses maps members to their API representations.
func ToMemberResponses(members []Member) []MemberResponse {
	out := make([]MemberResponse, 0, len(members))
	for _, member := range members {
		out = append(out, member.ToResponse())
	}

	return out
}
