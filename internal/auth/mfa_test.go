package auth_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"launchpad/internal/audit"
	"launchpad/internal/auth"
)

// --- in-memory MFA fakes ----------------------------------------------------

type fakeMFAStore struct {
	byID map[string]auth.MFAEnrollment
}

func newFakeMFAStore() *fakeMFAStore {
	return &fakeMFAStore{byID: map[string]auth.MFAEnrollment{}}
}

func (f *fakeMFAStore) EnsureIndexes(context.Context) error { return nil }

func (f *fakeMFAStore) Get(_ context.Context, organizationID, userID string) (auth.MFAEnrollment, error) {
	if enrollment, ok := f.byID[auth.MFAScopeID(organizationID, userID)]; ok {
		return enrollment, nil
	}

	return auth.MFAEnrollment{}, auth.ErrMFANotEnrolled
}

func (f *fakeMFAStore) Upsert(_ context.Context, enrollment auth.MFAEnrollment) error {
	f.byID[enrollment.ID] = enrollment

	return nil
}

func (f *fakeMFAStore) ConsumeBackupCode(
	_ context.Context,
	organizationID, userID, codeHash string,
) (bool, error) {
	id := auth.MFAScopeID(organizationID, userID)

	enrollment, ok := f.byID[id]
	if !ok {
		return false, nil
	}

	for index, hash := range enrollment.BackupCodeHashes {
		if hash == codeHash {
			enrollment.BackupCodeHashes = append(enrollment.BackupCodeHashes[:index], enrollment.BackupCodeHashes[index+1:]...)
			f.byID[id] = enrollment

			return true, nil
		}
	}

	return false, nil
}

func (f *fakeMFAStore) Delete(_ context.Context, organizationID, userID string) error {
	delete(f.byID, auth.MFAScopeID(organizationID, userID))

	return nil
}

type fakeMFATickets struct {
	byHash map[string]auth.MFATicket
}

func newFakeMFATickets() *fakeMFATickets {
	return &fakeMFATickets{byHash: map[string]auth.MFATicket{}}
}

func (f *fakeMFATickets) EnsureIndexes(context.Context) error { return nil }

func (f *fakeMFATickets) Save(_ context.Context, ticket auth.MFATicket) error {
	f.byHash[ticket.TicketHash] = ticket

	return nil
}

func (f *fakeMFATickets) Consume(_ context.Context, ticketHash string) (auth.MFATicket, error) {
	ticket, ok := f.byHash[ticketHash]
	if !ok || !ticket.ExpiresAt.After(time.Now().UTC()) {
		return auth.MFATicket{}, auth.ErrMFATicketInvalid
	}

	delete(f.byHash, ticketHash)

	return ticket, nil
}

type fakePlatformStaff struct {
	roles map[string]string
}

func (f fakePlatformStaff) GetByUserID(_ context.Context, userID string) (string, error) {
	if role, ok := f.roles[userID]; ok {
		return role, nil
	}

	return "", auth.ErrPlatformStaffNotFound
}

// testTOTP independently recomputes the current 6-digit RFC 6238 code so the
// tests verify the service implementation rather than mirror it.
func testTOTP(t *testing.T, secretBase32 string, at time.Time) string {
	t.Helper()

	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secretBase32)
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(at.Unix())/30)

	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff

	return fmt.Sprintf("%06d", value%1000000)
}

// newMFAService builds an auth service with the MFA stores attached.
func newMFAService(t *testing.T) *auth.Service {
	t.Helper()

	svc, _, _ := newAuthService(t)

	return svc.WithMFA(newFakeMFAStore(), newFakeMFATickets())
}

// newMFAServiceWithStores additionally exposes the user and enrollment fakes
// for tests that assert on stored state.
func newMFAServiceWithStores(t *testing.T) (*auth.Service, *fakeUsers, *fakeMFAStore) {
	t.Helper()

	svc, users, _ := newAuthService(t)
	mfa := newFakeMFAStore()

	return svc.WithMFA(mfa, newFakeMFATickets()), users, mfa
}

// enrollAndConfirm runs the full enrollment flow and returns the enrollment
// result (secret + backup codes).
func enrollAndConfirm(
	t *testing.T,
	svc *auth.Service,
	organizationID, userID string,
) auth.MFAEnrollResult {
	t.Helper()

	ctx := context.Background()

	enrolled, err := svc.MFAEnroll(ctx, organizationID, userID)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}

	if enrolled.Secret == "" || enrolled.OTPAuthURL == "" || len(enrolled.BackupCodes) != 8 {
		t.Fatalf("enroll must return the secret, otpauth URL, and 8 backup codes: %+v", enrolled)
	}

	if !strings.Contains(enrolled.OTPAuthURL, "secret="+enrolled.Secret) {
		t.Fatalf("otpauth URL must carry the secret: %s", enrolled.OTPAuthURL)
	}

	if err := svc.MFAConfirm(ctx, organizationID, userID, testTOTP(t, enrolled.Secret, time.Now().UTC())); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	return enrolled
}

// --- lifecycle ----------------------------------------------------------------

func TestMFAEnrollConfirmDisableLifecycle(t *testing.T) {
	t.Parallel()

	svc, users, mfa := newMFAServiceWithStores(t)
	ctx := context.Background()

	result := registerOwner(t, svc, "owner@acme.test")
	orgID := result.Organization.ID
	userID := result.User.ID

	enrolled, err := svc.MFAEnroll(ctx, orgID, userID)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}

	if err := svc.MFAConfirm(ctx, orgID, userID, "000000"); !errors.Is(err, auth.ErrMFACodeInvalid) {
		t.Fatalf("confirm with a wrong code got %v, want ErrMFACodeInvalid", err)
	}

	if err := svc.MFAConfirm(ctx, orgID, userID, testTOTP(t, enrolled.Secret, time.Now().UTC())); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	if !users.byID[userID].MFAEnabled {
		t.Fatal("user.MFAEnabled must be set after confirm")
	}

	if _, err := svc.MFAEnroll(ctx, orgID, userID); !errors.Is(err, auth.ErrMFAAlreadyEnabled) {
		t.Fatalf("re-enroll got %v, want ErrMFAAlreadyEnabled", err)
	}

	if err := svc.MFADisable(ctx, orgID, userID, "000000"); !errors.Is(err, auth.ErrMFACodeInvalid) {
		t.Fatalf("disable with a wrong code got %v, want ErrMFACodeInvalid", err)
	}

	if err := svc.MFADisable(ctx, orgID, userID, testTOTP(t, enrolled.Secret, time.Now().UTC())); err != nil {
		t.Fatalf("disable: %v", err)
	}

	if users.byID[userID].MFAEnabled {
		t.Fatal("user.MFAEnabled must be cleared after disable")
	}

	if _, err := mfa.Get(ctx, orgID, userID); !errors.Is(err, auth.ErrMFANotEnrolled) {
		t.Fatalf("enrollment must be deleted after disable, got %v", err)
	}
}

func TestMFAConfirmRequiresEnrollment(t *testing.T) {
	t.Parallel()

	svc := newMFAService(t)

	result := registerOwner(t, svc, "owner@acme.test")

	err := svc.MFAConfirm(context.Background(), result.Organization.ID, result.User.ID, "123456")
	if !errors.Is(err, auth.ErrMFANotEnrolled) {
		t.Fatalf("confirm without enrollment got %v, want ErrMFANotEnrolled", err)
	}
}

// --- login challenge ------------------------------------------------------------

func TestLoginReturnsMFARequiredWhenEnabled(t *testing.T) {
	t.Parallel()

	svc := newMFAService(t)
	ctx := context.Background()

	result := registerOwner(t, svc, "owner@acme.test")
	enrolled := enrollAndConfirm(t, svc, result.Organization.ID, result.User.ID)

	challenge, err := svc.Login(ctx, auth.LoginInput{Email: "owner@acme.test", Password: goodPass})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if !challenge.MFARequired || challenge.MFATicket == "" {
		t.Fatalf("login must return mfaRequired with a ticket: %+v", challenge)
	}

	if challenge.Tokens.AccessToken != "" {
		t.Fatal("an MFA-challenged login must not issue tokens")
	}

	completed, err := svc.CompleteMFALogin(ctx, challenge.MFATicket, testTOTP(t, enrolled.Secret, time.Now().UTC()))
	if err != nil {
		t.Fatalf("complete mfa login: %v", err)
	}

	if completed.Tokens.AccessToken == "" || completed.Tokens.RefreshToken == "" {
		t.Fatal("completed mfa login must issue a token pair")
	}

	if completed.Organization == nil || completed.Organization.ID != result.Organization.ID {
		t.Fatalf("completed login must carry the organization: %+v", completed.Organization)
	}
}

func TestMFATicketIsSingleUse(t *testing.T) {
	t.Parallel()

	svc := newMFAService(t)
	ctx := context.Background()

	result := registerOwner(t, svc, "owner@acme.test")
	enrolled := enrollAndConfirm(t, svc, result.Organization.ID, result.User.ID)

	challenge, err := svc.Login(ctx, auth.LoginInput{Email: "owner@acme.test", Password: goodPass})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	code := testTOTP(t, enrolled.Secret, time.Now().UTC())
	if _, err := svc.CompleteMFALogin(ctx, challenge.MFATicket, code); err != nil {
		t.Fatalf("first completion: %v", err)
	}

	if _, err := svc.CompleteMFALogin(ctx, challenge.MFATicket, code); !errors.Is(err, auth.ErrMFATicketInvalid) {
		t.Fatalf("ticket replay got %v, want ErrMFATicketInvalid", err)
	}
}

func TestMFAWrongCodeBurnsTicket(t *testing.T) {
	t.Parallel()

	svc := newMFAService(t)
	ctx := context.Background()

	result := registerOwner(t, svc, "owner@acme.test")
	enrolled := enrollAndConfirm(t, svc, result.Organization.ID, result.User.ID)

	challenge, err := svc.Login(ctx, auth.LoginInput{Email: "owner@acme.test", Password: goodPass})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if _, err := svc.CompleteMFALogin(ctx, challenge.MFATicket, "000000"); !errors.Is(err, auth.ErrMFACodeInvalid) {
		t.Fatalf("wrong code got %v, want ErrMFACodeInvalid", err)
	}

	// The ticket was consumed with the attempt: even a correct code can no
	// longer redeem it — the user signs in with their password again.
	_, err = svc.CompleteMFALogin(ctx, challenge.MFATicket, testTOTP(t, enrolled.Secret, time.Now().UTC()))
	if !errors.Is(err, auth.ErrMFATicketInvalid) {
		t.Fatalf("burned ticket got %v, want ErrMFATicketInvalid", err)
	}
}

// --- backup codes ---------------------------------------------------------------

func TestMFABackupCodeIsSingleUse(t *testing.T) {
	t.Parallel()

	svc := newMFAService(t)
	ctx := context.Background()

	result := registerOwner(t, svc, "owner@acme.test")
	enrolled := enrollAndConfirm(t, svc, result.Organization.ID, result.User.ID)

	challenge, err := svc.Login(ctx, auth.LoginInput{Email: "owner@acme.test", Password: goodPass})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if _, err := svc.CompleteMFALogin(ctx, challenge.MFATicket, enrolled.BackupCodes[0]); err != nil {
		t.Fatalf("backup code login: %v", err)
	}

	second, err := svc.Login(ctx, auth.LoginInput{Email: "owner@acme.test", Password: goodPass})
	if err != nil {
		t.Fatalf("second login: %v", err)
	}

	_, err = svc.CompleteMFALogin(ctx, second.MFATicket, enrolled.BackupCodes[0])
	if !errors.Is(err, auth.ErrMFACodeInvalid) {
		t.Fatalf("replayed backup code got %v, want ErrMFACodeInvalid", err)
	}

	// A different, unconsumed backup code still works.
	third, err := svc.Login(ctx, auth.LoginInput{Email: "owner@acme.test", Password: goodPass})
	if err != nil {
		t.Fatalf("third login: %v", err)
	}

	if _, err := svc.CompleteMFALogin(ctx, third.MFATicket, enrolled.BackupCodes[1]); err != nil {
		t.Fatalf("second backup code login: %v", err)
	}
}

// --- platform staff rule ----------------------------------------------------------

// TestPlatformStaffMFA verifies the chosen enforcement rule: staff WITHOUT an
// enrollment are not hard-locked (they get a settings prompt instead), while
// an enabled enrollment challenges every login — including the org-less
// platform-staff path.
func TestPlatformStaffMFA(t *testing.T) {
	t.Parallel()

	users := newFakeUsers()
	orgs := newStubOrgDirectory()
	mfa := newFakeMFAStore()
	tickets := newFakeMFATickets()
	staffRoles := map[string]string{}

	cfg := auth.Config{
		JWTSecret: "test-secret-value", AccessTTL: time.Minute, RefreshTTL: time.Hour,
		InviteTTL: time.Hour, PasswordMinLen: 10,
	}
	svc := auth.NewService(
		users, orgs, audit.NewService(noopAuditRepo{}), newFakeSessions(), newFakeInvites(), cfg,
		fakePlatformStaff{roles: staffRoles},
	).WithMFA(mfa, tickets)

	ctx := context.Background()

	user, err := svc.CreateUserAccount(ctx, "owner@platform.test", "Owner", goodPass)
	if err != nil {
		t.Fatalf("create staff account: %v", err)
	}

	staffRoles[user.ID] = "platform_owner"

	// Without an enrollment the staff login is not hard-locked.
	plain, err := svc.Login(ctx, auth.LoginInput{Email: "owner@platform.test", Password: goodPass})
	if err != nil {
		t.Fatalf("staff login without mfa must succeed: %v", err)
	}

	if plain.MFARequired || plain.Tokens.AccessToken == "" {
		t.Fatalf("staff login without enrollment must issue tokens: %+v", plain)
	}

	enrolled := enrollAndConfirm(t, svc, "", user.ID)

	challenge, err := svc.Login(ctx, auth.LoginInput{Email: "owner@platform.test", Password: goodPass})
	if err != nil {
		t.Fatalf("staff login: %v", err)
	}

	if !challenge.MFARequired {
		t.Fatal("staff login with MFA enabled must return mfaRequired")
	}

	completed, err := svc.CompleteMFALogin(ctx, challenge.MFATicket, testTOTP(t, enrolled.Secret, time.Now().UTC()))
	if err != nil {
		t.Fatalf("complete staff mfa login: %v", err)
	}

	if completed.Organization != nil {
		t.Fatal("platform staff login must not carry an organization")
	}

	if completed.Tokens.AccessToken == "" {
		t.Fatal("completed staff mfa login must issue tokens")
	}
}

// --- enrollment guards --------------------------------------------------------------

func TestMFAVerifyAcceptsDriftedCode(t *testing.T) {
	t.Parallel()

	svc := newMFAService(t)
	ctx := context.Background()

	result := registerOwner(t, svc, "owner@acme.test")
	orgID := result.Organization.ID
	userID := result.User.ID

	enrolled, err := svc.MFAEnroll(ctx, orgID, userID)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}

	// A code from the previous 30-second step must confirm (±1 drift window).
	previous := testTOTP(t, enrolled.Secret, time.Now().UTC().Add(-30*time.Second))
	if err := svc.MFAConfirm(ctx, orgID, userID, previous); err != nil {
		t.Fatalf("confirm with previous-step code: %v", err)
	}
}

func (f *fakeMFAStore) DeleteForOrganization(_ context.Context, organizationID string) (int64, error) {
	var count int64

	for key, enrollment := range f.byID {
		if enrollment.OrganizationID == organizationID {
			delete(f.byID, key)

			count++
		}
	}

	return count, nil
}
