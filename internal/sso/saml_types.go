package sso

import "time"

// SAMLConfig is the tenant-controlled identity-provider metadata and attribute
// mapping. LaunchPad's SP identity and certificate are process configuration.
type SAMLConfig struct {
	OrganizationID string    `bson:"_id" json:"organizationId"`
	Enabled        bool      `bson:"enabled" json:"enabled"`
	IDPMetadataXML string    `bson:"idpMetadataXml" json:"-"`
	EmailAttribute string    `bson:"emailAttribute" json:"emailAttribute"`
	UpdatedAt      time.Time `bson:"updatedAt" json:"updatedAt"`
}

type SetSAMLConfigInput struct {
	Enabled        bool
	IDPMetadataXML string
	EmailAttribute string
}

// SAMLConfigResponse omits raw metadata, which can contain provider-specific
// operational details, while exposing whether it has been configured.
type SAMLConfigResponse struct {
	OrganizationID string    `json:"organizationId"`
	Enabled        bool      `json:"enabled"`
	Configured     bool      `json:"configured"`
	EmailAttribute string    `json:"emailAttribute"`
	MetadataURL    string    `json:"metadataUrl"`
	ACSURL         string    `json:"acsUrl"`
	EntityID       string    `json:"entityId"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
