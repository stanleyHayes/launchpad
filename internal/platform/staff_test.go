package platform_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"launchpad/internal/platform"
)

// --- staff administration fakes ----------------------------------------------

type fakeAccountCreator struct {
	userID string
	err    error

	gotEmail       string
	gotDisplayName string
	gotPassword    string
}

func (f *fakeAccountCreator) CreateAccount(
	_ context.Context,
	email, displayName, password string,
) (string, error) {
	f.gotEmail = email
	f.gotDisplayName = displayName
	f.gotPassword = password

	if f.err != nil {
		return "", f.err
	}

	return f.userID, nil
}

type fakeMailSender struct {
	err error

	gotTo      string
	gotSubject string
	gotBody    string
}

func (f *fakeMailSender) Send(_ context.Context, to, subject, html string) error {
	f.gotTo = to
	f.gotSubject = subject
	f.gotBody = html

	return f.err
}

func newStaffService() (*platform.Service, *fakeStaffRepo, *fakeAccountCreator, *fakeMailSender) {
	repo := newFakeStaffRepo()
	orgs := newStubOrgDirectory()
	svc := platform.NewService(repo, orgs, stubLeadCounter{}, stubTicketCounter{})
	creator := &fakeAccountCreator{userID: "user-new"}
	mailer := &fakeMailSender{}

	return svc, repo, creator, mailer
}

func validStaffInput() platform.CreateStaffInput {
	return platform.CreateStaffInput{
		Email:       "  Agent@Example.com ",
		DisplayName: "Support Agent",
		RoleCode:    "support_agent",
	}
}

// --- tests ------------------------------------------------------------------

func TestCreateStaffReturnsTempPasswordWithoutMailer(t *testing.T) {
	t.Parallel()

	svc, repo, creator, _ := newStaffService()
	svc.WithAccounts(creator, nil)

	result, err := svc.CreateStaff(context.Background(), validStaffInput())
	if err != nil {
		t.Fatalf("create staff: %v", err)
	}

	if result.Invited || result.TempPassword == "" {
		t.Fatalf("without a mailer the temp password must be returned once, got %+v", result)
	}

	staff := result.Staff
	if staff.UserID != "user-new" || staff.RoleCode != "support_agent" || staff.Status != "active" {
		t.Fatalf("unexpected staff record: %+v", staff)
	}

	if staff.Email != "agent@example.com" {
		t.Fatalf("email must be normalized, got %q", staff.Email)
	}

	if creator.gotPassword != result.TempPassword {
		t.Fatal("account must be created with the generated temp password")
	}

	if _, ok := repo.staff["user-new"]; !ok {
		t.Fatal("staff record not persisted")
	}
}

func TestCreateStaffEmailsInviteWithMailer(t *testing.T) {
	t.Parallel()

	svc, _, creator, mailer := newStaffService()
	svc.WithAccounts(creator, mailer)

	result, err := svc.CreateStaff(context.Background(), validStaffInput())
	if err != nil {
		t.Fatalf("create staff: %v", err)
	}

	if !result.Invited || result.TempPassword != "" {
		t.Fatalf("with a mailer the temp password must not be returned, got %+v", result)
	}

	if mailer.gotTo != "agent@example.com" || !strings.Contains(mailer.gotBody, creator.gotPassword) {
		t.Fatalf("invite email must deliver the temp password, got to=%q", mailer.gotTo)
	}
}

func TestCreateStaffValidatesInput(t *testing.T) {
	t.Parallel()

	svc, _, creator, mailer := newStaffService()
	svc.WithAccounts(creator, mailer)

	for _, in := range []platform.CreateStaffInput{
		{Email: "", DisplayName: "Agent", RoleCode: "support_agent"},
		{Email: "agent@example.com", DisplayName: "", RoleCode: "support_agent"},
		{Email: "not-an-email", DisplayName: "Agent", RoleCode: "support_agent"},
		{Email: "agent@example.com", DisplayName: "Agent", RoleCode: "superuser"},
		{Email: "agent@example.com", DisplayName: "Agent", RoleCode: "organization_owner"},
	} {
		if _, err := svc.CreateStaff(context.Background(), in); !errors.Is(err, platform.ErrInvalidInput) {
			t.Fatalf("input %+v: got %v, want ErrInvalidInput", in, err)
		}
	}
}

func TestCreateStaffAcceptsFullRoleSet(t *testing.T) {
	t.Parallel()

	svc, _, creator, mailer := newStaffService()
	svc.WithAccounts(creator, mailer)

	for _, roleCode := range platform.RoleCodes() {
		input := validStaffInput()
		input.RoleCode = roleCode

		result, err := svc.CreateStaff(context.Background(), input)
		if err != nil {
			t.Fatalf("role %q: %v", roleCode, err)
		}

		if result.Staff.RoleCode != roleCode {
			t.Fatalf("role %q: stored %q", roleCode, result.Staff.RoleCode)
		}
	}
}

func TestCreateStaffRequiresProvisioning(t *testing.T) {
	t.Parallel()

	svc := platform.NewService(
		newFakeStaffRepo(),
		newStubOrgDirectory(),
		stubLeadCounter{},
		stubTicketCounter{},
	)

	_, err := svc.CreateStaff(context.Background(), validStaffInput())
	if !errors.Is(err, platform.ErrProvisioningUnavailable) {
		t.Fatalf("got %v, want ErrProvisioningUnavailable", err)
	}
}

func TestCreateStaffPropagatesAccountCreatorError(t *testing.T) {
	t.Parallel()

	svc, repo, creator, mailer := newStaffService()
	svc.WithAccounts(creator, mailer)
	creator.err = errors.New("email taken")

	if _, err := svc.CreateStaff(context.Background(), validStaffInput()); !errors.Is(err, creator.err) {
		t.Fatalf("got %v, want the account creator error", err)
	}

	if len(repo.staff) != 0 {
		t.Fatal("staff record must not be created when account creation fails")
	}
}

func TestListStaffReturnsAllRecords(t *testing.T) {
	t.Parallel()

	svc, repo, _, _ := newStaffService()

	repo.staff["u1"] = platform.Staff{ID: "s1", UserID: "u1", RoleCode: "platform_owner", Status: "active"}
	repo.staff["u2"] = platform.Staff{ID: "s2", UserID: "u2", RoleCode: "analyst", Status: "deactivated"}

	items, err := svc.ListStaff(context.Background())
	if err != nil {
		t.Fatalf("list staff: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("list must include deactivated records, got %d", len(items))
	}
}

func TestBreakGlassGrantChangesEffectiveRoleUntilRevoked(t *testing.T) {
	t.Parallel()
	svc, repo, _, _ := newStaffService()
	repo.staff["target"] = platform.Staff{
		ID: "s-target", UserID: "target", RoleCode: "support_agent", Status: "active",
	}
	granted, err := svc.GrantBreakGlass(t.Context(), "owner", "s-target", "incident response", 30*time.Minute)
	if err != nil {
		t.Fatalf("grant break glass: %v", err)
	}
	if granted.BreakGlass == nil || granted.BreakGlass.RoleCode != "platform_owner" {
		t.Fatalf("unexpected grant: %+v", granted)
	}
	effective, err := svc.GetByUserID(t.Context(), "target")
	if err != nil || effective.RoleCode != "platform_owner" {
		t.Fatalf("effective role not elevated: %+v err=%v", effective, err)
	}
	if _, err := svc.RevokeBreakGlass(t.Context(), "owner", "s-target"); err != nil {
		t.Fatalf("revoke break glass: %v", err)
	}
	effective, err = svc.GetByUserID(t.Context(), "target")
	if err != nil || effective.RoleCode != "support_agent" {
		t.Fatalf("effective role not restored: %+v err=%v", effective, err)
	}
}

func TestAccessReviewFlagsAndAttestsStaff(t *testing.T) {
	t.Parallel()
	svc, repo, _, _ := newStaffService()
	repo.staff["u1"] = platform.Staff{ID: "s1", UserID: "u1", RoleCode: "analyst", Status: "active"}
	items, err := svc.AccessReview(t.Context())
	if err != nil || len(items) != 1 || !items[0].ReviewDue {
		t.Fatalf("unexpected initial access review: %+v err=%v", items, err)
	}
	if _, err := svc.AttestAccess(t.Context(), "security", "s1"); err != nil {
		t.Fatalf("attest: %v", err)
	}
	items, err = svc.AccessReview(t.Context())
	if err != nil || items[0].ReviewDue || items[0].Staff.AccessReviewedBy != "security" {
		t.Fatalf("unexpected attested review: %+v err=%v", items, err)
	}
}

func TestUpdateStaffRole(t *testing.T) {
	t.Parallel()

	svc, repo, _, _ := newStaffService()
	ctx := context.Background()

	repo.staff["u1"] = platform.Staff{ID: "s1", UserID: "u1", RoleCode: "analyst", Status: "active"}

	updated, err := svc.UpdateStaffRole(ctx, "s1", "security_admin")
	if err != nil {
		t.Fatalf("update role: %v", err)
	}

	if updated.RoleCode != "security_admin" || repo.staff["u1"].RoleCode != "security_admin" {
		t.Fatalf("role change not applied: %+v", updated)
	}

	if _, err := svc.UpdateStaffRole(ctx, "s1", "superuser"); !errors.Is(err, platform.ErrInvalidInput) {
		t.Fatalf("unknown role got %v, want ErrInvalidInput", err)
	}

	if _, err := svc.UpdateStaffRole(ctx, "ghost", "analyst"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("unknown staff got %v, want ErrNotFound", err)
	}
}

func TestDeactivateStaffBlocksLoginPath(t *testing.T) {
	t.Parallel()

	svc, repo, _, _ := newStaffService()
	ctx := context.Background()

	repo.staff["u1"] = platform.Staff{ID: "s1", UserID: "u1", RoleCode: "analyst", Status: "active"}

	deactivated, err := svc.SetStaffStatus(ctx, "actor", "s1", "deactivated")
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	if deactivated.Status != "deactivated" {
		t.Fatalf("status not applied: %+v", deactivated)
	}

	// The staff login path resolves roles via GetByUserID; a deactivated
	// record must be invisible to it so login fails closed.
	if _, err := svc.GetByUserID(ctx, "u1"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("deactivated staff resolved by login path: %v, want ErrNotFound", err)
	}

	reactivated, err := svc.SetStaffStatus(ctx, "actor", "s1", "active")
	if err != nil {
		t.Fatalf("reactivate: %v", err)
	}

	if reactivated.Status != "active" {
		t.Fatalf("reactivation not applied: %+v", reactivated)
	}

	if _, err := svc.GetByUserID(ctx, "u1"); err != nil {
		t.Fatalf("reactivated staff must resolve again: %v", err)
	}
}

func TestSetStaffStatusRejectsSelfDeactivation(t *testing.T) {
	t.Parallel()

	svc, repo, _, _ := newStaffService()

	repo.staff["u1"] = platform.Staff{ID: "s1", UserID: "u1", RoleCode: "platform_owner", Status: "active"}

	_, err := svc.SetStaffStatus(context.Background(), "u1", "s1", "deactivated")
	if !errors.Is(err, platform.ErrInvalidInput) {
		t.Fatalf("self-deactivation got %v, want ErrInvalidInput", err)
	}

	if repo.staff["u1"].Status != "active" {
		t.Fatal("self-deactivation must not change the record")
	}

	_, err = svc.SetStaffStatus(context.Background(), "actor", "s1", "paused")
	if !errors.Is(err, platform.ErrInvalidInput) {
		t.Fatalf("unknown status got %v, want ErrInvalidInput", err)
	}
}
