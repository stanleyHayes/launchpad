package security_test

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"launchpad/pkg/security"
)

func newTestKey(t *testing.T) string {
	t.Helper()

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}

	return base64.StdEncoding.EncodeToString(key)
}

func TestSecretCipherRoundTrip(t *testing.T) {
	t.Parallel()

	cipher, err := security.NewSecretCipher(newTestKey(t))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	const secret = "xoxb-slack-webhook-secret"

	encrypted, err := cipher.Encrypt(secret)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if !strings.HasPrefix(encrypted, security.SecretCipherPrefix) {
		t.Fatalf("ciphertext missing prefix: %q", encrypted)
	}

	if strings.Contains(encrypted, secret) {
		t.Fatal("ciphertext leaks the plaintext")
	}

	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if decrypted != secret {
		t.Fatalf("decrypted = %q, want %q", decrypted, secret)
	}
}

func TestSecretCipherEncryptEmptyStaysEmpty(t *testing.T) {
	t.Parallel()

	cipher, err := security.NewSecretCipher(newTestKey(t))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	encrypted, err := cipher.Encrypt("")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if encrypted != "" {
		t.Fatalf("encrypted empty = %q, want empty", encrypted)
	}
}

func TestSecretCipherDecryptPassesThroughPlaintext(t *testing.T) {
	t.Parallel()

	cipher, err := security.NewSecretCipher(newTestKey(t))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	const legacy = "plaintext-token-written-before-encryption"

	decrypted, err := cipher.Decrypt(legacy)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if decrypted != legacy {
		t.Fatalf("decrypted = %q, want passthrough %q", decrypted, legacy)
	}
}

func TestSecretCipherRejectsWrongKey(t *testing.T) {
	t.Parallel()

	cipherA, err := security.NewSecretCipher(newTestKey(t))
	if err != nil {
		t.Fatalf("new cipher A: %v", err)
	}

	cipherB, err := security.NewSecretCipher(newTestKey(t))
	if err != nil {
		t.Fatalf("new cipher B: %v", err)
	}

	encrypted, err := cipherA.Encrypt("tenant-secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if _, err := cipherB.Decrypt(encrypted); err == nil {
		t.Fatal("expected decryption with a different key to fail")
	}
}

func TestNewSecretCipherRejectsBadKeys(t *testing.T) {
	t.Parallel()

	if _, err := security.NewSecretCipher("not-base64!!!"); err == nil {
		t.Fatal("expected invalid base64 to fail")
	}

	short := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := security.NewSecretCipher(short); err == nil {
		t.Fatal("expected non-32-byte key to fail")
	}
}

func TestPackageLevelSecretsPassThroughWithoutPrefix(t *testing.T) {
	// Regardless of key configuration, values without the enc:v1: prefix are
	// pre-existing plaintext and must pass through unchanged.
	t.Parallel()

	const legacy = "https://hooks.slack.com/services/legacy-plaintext"

	decrypted, err := security.DecryptSecret(legacy)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if decrypted != legacy {
		t.Fatalf("decrypted = %q, want passthrough %q", decrypted, legacy)
	}
}
