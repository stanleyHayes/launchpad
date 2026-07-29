package auth

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

// rfc6238TestSecret is the RFC 6238 Appendix B SHA-1 test secret (ASCII).
func rfc6238TestSecret() []byte {
	return []byte("12345678901234567890")
}

// TestTOTPCodeRFC6238Vectors checks the implementation against the RFC 6238
// Appendix B HMAC-SHA1 test vectors. The RFC specifies 8-digit codes; the
// 6-digit form LaunchPad uses must equal the low 6 digits of the same value.
func TestTOTPCodeRFC6238Vectors(t *testing.T) {
	t.Parallel()

	vectors := map[int64]string{
		59:          "94287082",
		1111111109:  "07081804",
		1111111111:  "14050471",
		1234567890:  "89005924",
		2000000000:  "69279037",
		20000000000: "65353130",
	}

	for unixSeconds, want := range vectors {
		counter := uint64(unixSeconds) / uint64(totpStep.Seconds())

		if got := totpCode(rfc6238TestSecret(), counter, 8); got != want {
			t.Errorf("T=%d 8-digit: got %s, want %s", unixSeconds, got, want)
		}

		if got := totpCode(rfc6238TestSecret(), counter, totpDigits); got != want[2:] {
			t.Errorf("T=%d 6-digit: got %s, want %s", unixSeconds, got, want[2:])
		}
	}
}

func TestVerifyTOTPAcceptsOneStepDrift(t *testing.T) {
	t.Parallel()

	at := time.Unix(1111111111, 0).UTC()
	current := totpCode(rfc6238TestSecret(), totpCounter(at), totpDigits)
	previous := totpCode(rfc6238TestSecret(), totpCounter(at)-1, totpDigits)
	next := totpCode(rfc6238TestSecret(), totpCounter(at)+1, totpDigits)
	outside := totpCode(rfc6238TestSecret(), totpCounter(at)-2, totpDigits)

	switch outside {
	case current, previous, next:
		t.Skip("vector collision makes the out-of-window code indistinguishable")
	}

	for name, code := range map[string]string{
		"current":  current,
		"previous": previous,
		"next":     next,
	} {
		if !verifyTOTP(rfc6238TestSecret(), code, at) {
			t.Errorf("%s step code %s must verify", name, code)
		}
	}

	if verifyTOTP(rfc6238TestSecret(), outside, at) {
		t.Errorf("code two steps out (%s) must not verify", outside)
	}

	if verifyTOTP(rfc6238TestSecret(), "12345", at) {
		t.Error("a 5-digit code must not verify")
	}
}

func TestNewTOTPSecretEncoding(t *testing.T) {
	t.Parallel()

	secret, err := newTOTPSecret()
	if err != nil {
		t.Fatalf("newTOTPSecret: %v", err)
	}

	// 20 bytes base32-encode to 32 characters without padding.
	if len(secret) != 32 {
		t.Fatalf("secret length = %d, want 32", len(secret))
	}

	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatalf("secret is not valid unpadded base32: %v", err)
	}

	if len(raw) != totpSecretBytes {
		t.Fatalf("decoded secret = %d bytes, want %d", len(raw), totpSecretBytes)
	}

	other, err := newTOTPSecret()
	if err != nil {
		t.Fatalf("newTOTPSecret: %v", err)
	}

	if strings.EqualFold(secret, other) {
		t.Fatal("two generated secrets must differ")
	}
}

func TestNormalizeBackupCode(t *testing.T) {
	t.Parallel()

	if got := normalizeBackupCode(" abcd-efgh "); got != "ABCDEFGH" {
		t.Fatalf("normalizeBackupCode = %q, want ABCDEFGH", got)
	}
}

func TestOTPAuthURL(t *testing.T) {
	t.Parallel()

	got := otpauthURL("JBSWY3DPEHPK3PXP", "User@Acme.test")

	for _, want := range []string{
		"otpauth://totp/",
		"secret=JBSWY3DPEHPK3PXP",
		"issuer=LaunchPad",
		"digits=6",
		"period=30",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("otpauth URL %q missing %q", got, want)
		}
	}
}
