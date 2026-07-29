package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 mandates HMAC-SHA1 for TOTP authenticator interoperability.
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"launchpad/pkg/security"
)

// errMFANotConfigured indicates the MFA stores were never wired (WithMFA not
// called).
var errMFANotConfigured = errors.New("mfa is not configured")

const (
	// totpSecretBytes is the RFC 4226/6238 recommended 160-bit shared secret.
	totpSecretBytes = 20
	// totpStep is the RFC 6238 default time step.
	totpStep = 30 * time.Second
	// totpDigits is the authenticator-standard 6-digit code length.
	totpDigits = 6
	// totpDriftSteps accepts codes one step either side of the current one to
	// tolerate authenticator clock drift.
	totpDriftSteps = 1

	totpCounterBytes = 8
	totpMACWordBytes = 4
	totpOffsetMask   = 0x0f
	totpValueMask    = 0x7fffffff

	// mfaBackupCodeCount is how many one-time recovery codes an enrollment
	// issues; mfaBackupCodeBytes of entropy each (base32 → 8 characters).
	mfaBackupCodeCount = 8
	mfaBackupCodeBytes = 5

	// mfaTicketTTL bounds how long a password-verified login may linger
	// waiting for the second factor.
	mfaTicketTTL    = 5 * time.Minute
	mfaTicketPrefix = "mfa_"

	mfaIssuer = "LaunchPad"

	actionMFAEnrolled  = "auth.mfa.enrolled"
	actionMFAEnabled   = "auth.mfa.enabled"
	actionMFADisabled  = "auth.mfa.disabled"
	actionMFAChallenge = "auth.mfa.challenge"
)

// MFAEnrollment is a user's TOTP enrollment within one organization scope (an
// empty OrganizationID is the platform-staff scope). The secret is stored
// encrypted; backup codes are stored hashed and are single-use.
type MFAEnrollment struct {
	ID               string    `bson:"_id"              json:"-"`
	UserID           string    `bson:"userId"           json:"-"`
	OrganizationID   string    `bson:"organizationId"   json:"-"`
	SecretEnc        string    `bson:"secretEnc"        json:"-"`
	Enabled          bool      `bson:"enabled"          json:"-"`
	BackupCodeHashes []string  `bson:"backupCodeHashes" json:"-"`
	CreatedAt        time.Time `bson:"createdAt"        json:"-"`
	UpdatedAt        time.Time `bson:"updatedAt"        json:"-"`
}

// MFATicket is a short-lived, single-use grant created when a password-verified
// login still owes the second factor. It carries the resolved login context so
// completing the challenge can issue tokens without re-resolving membership.
type MFATicket struct {
	ID             string    `bson:"_id"`
	TicketHash     string    `bson:"ticketHash"`
	UserID         string    `bson:"userId"`
	OrganizationID string    `bson:"organizationId"`
	RoleCode       string    `bson:"roleCode"`
	ExpiresAt      time.Time `bson:"expiresAt"`
	CreatedAt      time.Time `bson:"createdAt"`
}

// MFAEnrollResult is returned once at enrollment; the secret and backup codes
// are never retrievable again (only the encrypted secret and code hashes are
// stored).
type MFAEnrollResult struct {
	Secret      string   `json:"secret"`
	OTPAuthURL  string   `json:"otpauthUrl"`
	BackupCodes []string `json:"backupCodes"`
}

// MFAScopeID builds the enrollment document id for the (organization, user)
// scope. Exported so the Mongo adapter filters on the same key the service
// writes.
func MFAScopeID(organizationID, userID string) string {
	return organizationID + "|" + userID
}

// MFAEnroll starts TOTP enrollment: it generates a fresh secret and backup
// codes and stores them with Enabled=false until MFAConfirm proves the
// authenticator is provisioned. Re-enrolling overwrites a pending (never
// confirmed) enrollment; an enabled one must be disabled first.
func (s *Service) MFAEnroll(ctx context.Context, organizationID, userID string) (MFAEnrollResult, error) {
	if s.mfa == nil {
		return MFAEnrollResult{}, errMFANotConfigured
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return MFAEnrollResult{}, fmt.Errorf("load user: %w", err)
	}

	if err := s.requireMFAReenrollable(ctx, organizationID, userID); err != nil {
		return MFAEnrollResult{}, err
	}

	secret, err := newTOTPSecret()
	if err != nil {
		return MFAEnrollResult{}, err
	}

	secretEnc, err := security.EncryptSecret(secret)
	if err != nil {
		return MFAEnrollResult{}, fmt.Errorf("encrypt mfa secret: %w", err)
	}

	codes, hashes, err := newBackupCodes()
	if err != nil {
		return MFAEnrollResult{}, err
	}

	now := time.Now().UTC()
	if err := s.mfa.Upsert(ctx, MFAEnrollment{
		ID:               MFAScopeID(organizationID, userID),
		UserID:           userID,
		OrganizationID:   organizationID,
		SecretEnc:        secretEnc,
		Enabled:          false,
		BackupCodeHashes: hashes,
		CreatedAt:        now,
		UpdatedAt:        now,
	}); err != nil {
		return MFAEnrollResult{}, fmt.Errorf("save mfa enrollment: %w", err)
	}

	if err := s.recordMFA(ctx, organizationID, userID, actionMFAEnrolled); err != nil {
		return MFAEnrollResult{}, err
	}

	return MFAEnrollResult{
		Secret:      secret,
		OTPAuthURL:  otpauthURL(secret, user.Email),
		BackupCodes: codes,
	}, nil
}

// MFAConfirm enables a pending enrollment once the authenticator produces a
// valid TOTP code (±1 step drift).
func (s *Service) MFAConfirm(ctx context.Context, organizationID, userID, code string) error {
	if s.mfa == nil {
		return errMFANotConfigured
	}

	enrollment, err := s.mfa.Get(ctx, organizationID, userID)
	if err != nil {
		return fmt.Errorf("load mfa enrollment: %w", err)
	}

	if enrollment.Enabled {
		return ErrMFAAlreadyEnabled
	}

	secret, err := decryptTOTPSecret(enrollment)
	if err != nil {
		return err
	}

	if !verifyTOTP(secret, normalizeTOTPCode(code), time.Now().UTC()) {
		return ErrMFACodeInvalid
	}

	enrollment.Enabled = true
	enrollment.UpdatedAt = time.Now().UTC()

	if err := s.mfa.Upsert(ctx, enrollment); err != nil {
		return fmt.Errorf("enable mfa enrollment: %w", err)
	}

	if err := s.setUserMFAEnabled(ctx, userID, true); err != nil {
		return err
	}

	return s.recordMFA(ctx, organizationID, userID, actionMFAEnabled)
}

// MFADisable removes the enrollment. It requires a valid TOTP or backup code
// so a hijacked session alone cannot strip the account's second factor.
func (s *Service) MFADisable(ctx context.Context, organizationID, userID, code string) error {
	if s.mfa == nil {
		return errMFANotConfigured
	}

	enrollment, err := s.mfa.Get(ctx, organizationID, userID)
	if err != nil {
		return fmt.Errorf("load mfa enrollment: %w", err)
	}

	if !enrollment.Enabled {
		return ErrMFANotEnrolled
	}

	if err := s.verifyMFACode(ctx, enrollment, code); err != nil {
		return err
	}

	if err := s.mfa.Delete(ctx, organizationID, userID); err != nil {
		return fmt.Errorf("delete mfa enrollment: %w", err)
	}

	if err := s.setUserMFAEnabled(ctx, userID, false); err != nil {
		return err
	}

	return s.recordMFA(ctx, organizationID, userID, actionMFADisabled)
}

// CompleteMFALogin exchanges a single-use MFA ticket plus a TOTP or backup
// code for a full session. The ticket is consumed BEFORE the code is checked
// (same posture as password-reset tokens): a wrong code burns the ticket and
// the user signs in with their password again, which also bounds online
// guessing of the 6-digit code.
func (s *Service) CompleteMFALogin(ctx context.Context, ticket, code string) (Result, error) {
	ticket = strings.TrimSpace(ticket)
	if ticket == "" || s.mfa == nil || s.mfaTickets == nil {
		return Result{}, ErrMFATicketInvalid
	}

	grant, err := s.mfaTickets.Consume(ctx, security.HashToken(ticket))
	if err != nil {
		return Result{}, ErrMFATicketInvalid
	}

	user, err := s.users.GetByID(ctx, grant.UserID)
	if err != nil {
		return Result{}, ErrMFATicketInvalid
	}

	enrollment, err := s.mfa.Get(ctx, grant.OrganizationID, grant.UserID)
	if err != nil || !enrollment.Enabled {
		return Result{}, ErrMFACodeInvalid
	}

	if err := s.verifyMFACode(ctx, enrollment, code); err != nil {
		return Result{}, err
	}

	organization, err := s.mfaLoginOrganization(ctx, grant.OrganizationID)
	if err != nil {
		return Result{}, err
	}

	if err := s.audit.Record(ctx, auditOrgPtr(grant.OrganizationID), user.ID, "auth.login", "user", user.ID,
		map[string]any{"mfa": true}); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}

	tokens, err := s.issueTokens(ctx, user, grant.OrganizationID, grant.RoleCode)
	if err != nil {
		return Result{}, fmt.Errorf("issue mfa login tokens: %w", err)
	}

	return Result{User: toPublic(user), Organization: organization, Tokens: tokens}, nil
}

// challengeMFA records the second-factor challenge and returns the
// mfaRequired result holding a single-use ticket instead of tokens.
func (s *Service) challengeMFA(ctx context.Context, user User, orgID, roleCode string) (Result, error) {
	raw, err := security.NewRefreshToken()
	if err != nil {
		return Result{}, fmt.Errorf("generate mfa ticket: %w", err)
	}

	ticket := mfaTicketPrefix + raw
	now := time.Now().UTC()

	if err := s.mfaTickets.Save(ctx, MFATicket{
		ID:             uuid.NewString(),
		TicketHash:     security.HashToken(ticket),
		UserID:         user.ID,
		OrganizationID: orgID,
		RoleCode:       roleCode,
		ExpiresAt:      now.Add(mfaTicketTTL),
		CreatedAt:      now,
	}); err != nil {
		return Result{}, fmt.Errorf("save mfa ticket: %w", err)
	}

	if err := s.recordMFA(ctx, orgID, user.ID, actionMFAChallenge); err != nil {
		return Result{}, err
	}

	return Result{MFARequired: true, MFATicket: ticket}, nil
}

// mfaRequiredFor reports whether the user has an enabled MFA enrollment in the
// scope. Store errors fail CLOSED (the login is rejected): the user store was
// already read successfully, so an MFA-store failure is a real fault and must
// not silently strip the second factor.
func (s *Service) mfaRequiredFor(ctx context.Context, organizationID, userID string) (bool, error) {
	if s.mfa == nil {
		return false, nil
	}

	enrollment, err := s.mfa.Get(ctx, organizationID, userID)
	if errors.Is(err, ErrMFANotEnrolled) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("load mfa enrollment: %w", err)
	}

	return enrollment.Enabled, nil
}

// verifyMFACode accepts a current TOTP code (±1 step drift) or a single-use
// backup code, which is consumed atomically.
func (s *Service) verifyMFACode(ctx context.Context, enrollment MFAEnrollment, code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrMFACodeInvalid
	}

	secret, err := decryptTOTPSecret(enrollment)
	if err != nil {
		return err
	}

	if verifyTOTP(secret, normalizeTOTPCode(code), time.Now().UTC()) {
		return nil
	}

	consumed, err := s.mfa.ConsumeBackupCode(
		ctx, enrollment.OrganizationID, enrollment.UserID, security.HashToken(normalizeBackupCode(code)),
	)
	if err != nil {
		return fmt.Errorf("consume backup code: %w", err)
	}

	if !consumed {
		return ErrMFACodeInvalid
	}

	return nil
}

func (s *Service) requireMFAReenrollable(ctx context.Context, organizationID, userID string) error {
	existing, err := s.mfa.Get(ctx, organizationID, userID)
	if errors.Is(err, ErrMFANotEnrolled) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("load mfa enrollment: %w", err)
	}

	if existing.Enabled {
		return ErrMFAAlreadyEnabled
	}

	return nil
}

func (s *Service) setUserMFAEnabled(ctx context.Context, userID string, enabled bool) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("load user: %w", err)
	}

	user.MFAEnabled = enabled
	user.UpdatedAt = time.Now().UTC()

	if err := s.users.Update(ctx, user); err != nil {
		return fmt.Errorf("update user mfa flag: %w", err)
	}

	return nil
}

// mfaLoginOrganization loads the login organization for the response; the
// platform-staff scope (empty id) has none.
func (s *Service) mfaLoginOrganization(ctx context.Context, organizationID string) (*OrganizationPublic, error) {
	if organizationID == "" {
		//nolint:nilnil // platform staff legitimately have no organization.
		return nil, nil
	}

	org, err := s.orgs.Get(ctx, organizationID)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	return toOrganizationPublicPtr(org), nil
}

// recordMFA audits an MFA lifecycle event. Enrollment, enable, disable, and
// challenge are all privileged security actions, so a broken audit store
// fails the mutation (consistent with login/register auditing).
func (s *Service) recordMFA(ctx context.Context, organizationID, userID, action string) error {
	if err := s.audit.Record(ctx, auditOrgPtr(organizationID), userID, action, "user", userID, nil); err != nil {
		return fmt.Errorf("%w: %w", ErrAuditFailed, err)
	}

	return nil
}

// auditOrgPtr maps the platform-staff scope ("") to a nil organization.
func auditOrgPtr(organizationID string) *string {
	if organizationID == "" {
		return nil
	}

	return &organizationID
}

// newTOTPSecret generates a random 160-bit secret and returns it base32-encoded
// (no padding), the canonical form for otpauth:// URIs.
func newTOTPSecret() (string, error) {
	raw := make([]byte, totpSecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate mfa secret: %w", err)
	}

	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

// decryptTOTPSecret opens the stored secret and base32-decodes it.
func decryptTOTPSecret(enrollment MFAEnrollment) ([]byte, error) {
	encoded, err := security.DecryptSecret(enrollment.SecretEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt mfa secret: %w", err)
	}

	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode mfa secret: %w", err)
	}

	return raw, nil
}

// newBackupCodes issues one-time recovery codes, returning the display form
// (XXXX-XXXX, shown once) alongside the stored hashes of the normalized form.
func newBackupCodes() ([]string, []string, error) {
	codes := make([]string, 0, mfaBackupCodeCount)
	hashes := make([]string, 0, mfaBackupCodeCount)

	for range mfaBackupCodeCount {
		raw := make([]byte, mfaBackupCodeBytes)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, fmt.Errorf("generate backup code: %w", err)
		}

		normalized := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
		codes = append(codes, normalized[:totpMACWordBytes]+"-"+normalized[totpMACWordBytes:])
		hashes = append(hashes, security.HashToken(normalized))
	}

	return codes, hashes, nil
}

// normalizeTOTPCode strips whitespace; leading zeros are significant.
func normalizeTOTPCode(code string) string {
	return strings.ReplaceAll(strings.TrimSpace(code), " ", "")
}

// normalizeBackupCode maps a typed backup code to its hashed canonical form
// (uppercase, no dash), so users may enter it with or without formatting.
func normalizeBackupCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
}

// otpauthURL builds the provisioning URI authenticator apps consume (directly
// or via a QR code rendered client-side).
func otpauthURL(secret, email string) string {
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", mfaIssuer)
	query.Set("digits", strconv.Itoa(totpDigits))
	query.Set("period", strconv.Itoa(int(totpStep.Seconds())))

	return "otpauth://totp/" + url.PathEscape(mfaIssuer+":"+email) + "?" + query.Encode()
}

// totpCounter converts a time to the TOTP step counter.
func totpCounter(at time.Time) uint64 {
	return uint64(max(at.Unix(), 0)) / uint64(totpStep.Seconds())
}

// totpCode computes the RFC 6238 TOTP value (HMAC-SHA1 with RFC 4226 dynamic
// truncation) for the given counter with the given digit count.
func totpCode(secret []byte, counter uint64, digits int) string {
	var buf [totpCounterBytes]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(buf[:]) // hash.Hash Write never fails.
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & totpOffsetMask
	value := binary.BigEndian.Uint32(sum[offset:offset+totpMACWordBytes]) & totpValueMask

	modulus := uint32(1)
	for range digits {
		modulus *= 10
	}

	return fmt.Sprintf("%0*d", digits, value%modulus)
}

// verifyTOTP reports whether code matches the secret within ±totpDriftSteps
// steps of at, comparing each candidate in constant time.
func verifyTOTP(secret []byte, code string, at time.Time) bool {
	if len(code) != totpDigits {
		return false
	}

	counter := int64(totpCounter(at)) //nolint:gosec // step counters stay far below int64 overflow.
	for drift := -totpDriftSteps; drift <= totpDriftSteps; drift++ {
		candidate := counter + int64(drift)
		if candidate < 0 {
			continue
		}

		if subtle.ConstantTimeCompare([]byte(totpCode(secret, uint64(candidate), totpDigits)), []byte(code)) == 1 {
			return true
		}
	}

	return false
}
