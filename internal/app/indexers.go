package app

import (
	"context"

	drivermongo "go.mongodb.org/mongo-driver/v2/mongo"

	assessmentsmongo "launchpad/internal/assessments/mongo"
	assignmentsmongo "launchpad/internal/assignments/mongo"
	assistantmongo "launchpad/internal/assistant/mongo"
	auditmongo "launchpad/internal/audit/mongo"
	authmongo "launchpad/internal/auth/mongo"
	billingmongo "launchpad/internal/billing/mongo"
	cmsmongo "launchpad/internal/cms/mongo"
	departmentsmongo "launchpad/internal/departments/mongo"
	employeesmongo "launchpad/internal/employees/mongo"
	featureflagsmongo "launchpad/internal/featureflags/mongo"
	hrismongo "launchpad/internal/hris/mongo"
	integrationsmongo "launchpad/internal/integrations/mongo"
	journeysmongo "launchpad/internal/journeys/mongo"
	knowledgemongo "launchpad/internal/knowledge/mongo"
	leadsmongo "launchpad/internal/leads/mongo"
	marketplacemongo "launchpad/internal/marketplace/mongo"
	meetingsmongo "launchpad/internal/meetings/mongo"
	notificationsmongo "launchpad/internal/notifications/mongo"
	organizationsmongo "launchpad/internal/organizations/mongo"
	platformmongo "launchpad/internal/platform/mongo"
	requestsmongo "launchpad/internal/requests/mongo"
	rolesmongo "launchpad/internal/roles/mongo"
	scimmongo "launchpad/internal/scim/mongo"
	ssomongo "launchpad/internal/sso/mongo"
	supportmongo "launchpad/internal/support/mongo"
	supportsessionsmongo "launchpad/internal/supportsessions/mongo"
)

// NamedIndexer couples a human-readable name with an index initializer.
type NamedIndexer struct {
	Name   string
	Ensure func(context.Context) error
}

// MongoIndexers is the single registry of Mongo index initializers, shared by
// startup (ensureIndexes) and scripts/migrate_indexes so the two can never
// drift apart.
func MongoIndexers(db *drivermongo.Database) []NamedIndexer {
	return []NamedIndexer{
		{Name: "audit", Ensure: auditmongo.NewStore(db).EnsureIndexes},
		{Name: "organization", Ensure: organizationsmongo.NewStore(db).EnsureIndexes},
		{Name: "roles", Ensure: rolesmongo.NewStore(db).EnsureIndexes},
		{Name: "user", Ensure: authmongo.NewUserStore(db).EnsureIndexes},
		{Name: "auth-invitations", Ensure: authmongo.NewInvitationStore(db).EnsureIndexes},
		{Name: "auth-password-resets", Ensure: authmongo.NewPasswordResetStore(db).EnsureIndexes},
		{Name: "auth-mfa", Ensure: authmongo.NewMFAStore(db).EnsureIndexes},
		{Name: "auth-mfa-tickets", Ensure: authmongo.NewMFATicketStore(db).EnsureIndexes},
		{Name: "department", Ensure: departmentsmongo.NewStore(db).EnsureIndexes},
		{Name: "employee", Ensure: employeesmongo.NewStore(db).EnsureIndexes},
		{Name: "journey", Ensure: journeysmongo.NewStore(db).EnsureIndexes},
		{Name: "assignment", Ensure: assignmentsmongo.NewStore(db).EnsureIndexes},
		{Name: "notification", Ensure: notificationsmongo.NewStore(db).EnsureIndexes},
		{Name: "notification-delivery", Ensure: notificationsmongo.NewDeliveryStore(db).EnsureIndexes},
		{Name: "platform", Ensure: platformmongo.NewStore(db).EnsureIndexes},
		{Name: "leads", Ensure: leadsmongo.NewStore(db).EnsureIndexes},
		{Name: "marketplace", Ensure: marketplacemongo.NewStore(db).EnsureIndexes},
		{Name: "featureflags", Ensure: featureflagsmongo.NewStore(db).EnsureIndexes},
		{Name: "billing", Ensure: billingmongo.NewStore(db).EnsureIndexes},
		{Name: "support", Ensure: supportmongo.NewStore(db).EnsureIndexes},
		{Name: "support-sessions", Ensure: supportsessionsmongo.NewStore(db).EnsureIndexes},
		{Name: "requests", Ensure: requestsmongo.NewStore(db).EnsureIndexes},
		{Name: "cms", Ensure: cmsmongo.NewStore(db).EnsureIndexes},
		{Name: "knowledge", Ensure: knowledgemongo.NewStore(db).EnsureIndexes},
		{Name: "assessments", Ensure: assessmentsmongo.NewStore(db).EnsureIndexes},
		{Name: "meetings", Ensure: meetingsmongo.NewStore(db).EnsureIndexes},
		{Name: "calendar-connections", Ensure: meetingsmongo.NewConnectionStore(db).EnsureIndexes},
		{Name: "assistant-chunks", Ensure: assistantmongo.NewVectorStore(db).EnsureIndexes},
		{Name: "assistant-interactions", Ensure: assistantmongo.NewConversationStore(db).EnsureIndexes},
		{Name: "scim-users", Ensure: scimmongo.NewStore(db).EnsureIndexes},
		{Name: "scim-tokens", Ensure: scimmongo.NewTokenStore(db).EnsureIndexes},
		{Name: "scim-groups", Ensure: scimmongo.NewGroupStore(db).EnsureIndexes},
		{Name: "sso", Ensure: ssomongo.NewStore(db).EnsureIndexes},
		{Name: "saml", Ensure: ssomongo.NewSAMLStore(db).EnsureIndexes},
		{Name: "hris-config", Ensure: hrismongo.NewConfigStore(db).EnsureIndexes},
		{Name: "hris-state", Ensure: hrismongo.NewStore(db).EnsureIndexes},
		{Name: "integrations", Ensure: integrationsmongo.NewStore(db).EnsureIndexes},
	}
}
