package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

const (
	// SecretCipherPrefix marks values encrypted by SecretCipher so stored
	// ciphertext is distinguishable from pre-existing plaintext.
	SecretCipherPrefix = "enc:v1:"

	secretKeyBytes   = 32 // AES-256
	encryptionKeyEnv = "ENCRYPTION_KEY"
)

var (
	errSecretKeyLength  = errors.New("encryption key must be base64 of 32 bytes (AES-256)")
	errSecretCiphertext = errors.New("invalid encrypted secret")
	errSecretKeyMissing = errors.New("ENCRYPTION_KEY is required to decrypt a stored secret")
)

// SecretCipher envelope-encrypts tenant secrets (HRIS API tokens, SSO client
// secrets, webhook URLs) with AES-256-GCM. The nonce is prepended to the
// ciphertext and the result is base64-encoded behind SecretCipherPrefix.
type SecretCipher struct {
	aead cipher.AEAD
}

// NewSecretCipher constructs a SecretCipher from a base64-encoded 32-byte key
// (generate with `openssl rand -base64 32`).
func NewSecretCipher(keyBase64 string) (*SecretCipher, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(keyBase64))
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}

	if len(key) != secretKeyBytes {
		return nil, errSecretKeyLength
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init encryption cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init encryption gcm: %w", err)
	}

	return &SecretCipher{aead: aead}, nil
}

// Encrypt seals a plaintext secret. Empty input stays empty so "not
// configured" checks keep working.
func (c *SecretCipher) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate encryption nonce: %w", err)
	}

	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)

	return SecretCipherPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt opens a sealed secret. Values without SecretCipherPrefix (written
// before encryption was enabled) pass through unchanged.
func (c *SecretCipher) Decrypt(value string) (string, error) {
	if !strings.HasPrefix(value, SecretCipherPrefix) {
		return value, nil
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, SecretCipherPrefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted secret: %w", err)
	}

	if len(raw) < c.aead.NonceSize() {
		return "", errSecretCiphertext
	}

	plaintext, err := c.aead.Open(nil, raw[:c.aead.NonceSize()], raw[c.aead.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}

	return string(plaintext), nil
}

//nolint:gochecknoglobals // process-default cipher is built once from the environment
var (
	defaultCipherOnce sync.Once
	defaultCipher     *SecretCipher
	errDefaultCipher  error
)

// loadDefaultCipher builds the process-default cipher from ENCRYPTION_KEY,
// once. When the variable is unset it logs a single warning and returns a nil
// cipher, keeping the historical plaintext behavior.
func loadDefaultCipher() (*SecretCipher, error) {
	defaultCipherOnce.Do(func() {
		key := strings.TrimSpace(os.Getenv(encryptionKeyEnv))
		if key == "" {
			slog.Warn("ENCRYPTION_KEY is not set; tenant secrets are stored unencrypted at rest")

			return
		}

		defaultCipher, errDefaultCipher = NewSecretCipher(key)
	})

	return defaultCipher, errDefaultCipher
}

// EncryptSecret encrypts a tenant secret with the process-default cipher
// (ENCRYPTION_KEY). When the key is unset the value is returned unchanged.
func EncryptSecret(plaintext string) (string, error) {
	sc, err := loadDefaultCipher()
	if err != nil {
		return "", err
	}

	if sc == nil {
		return plaintext, nil
	}

	return sc.Encrypt(plaintext)
}

// DecryptSecret decrypts a tenant secret with the process-default cipher.
// Values without SecretCipherPrefix pass through unchanged, so pre-existing
// plaintext rows keep working with or without a key configured.
func DecryptSecret(value string) (string, error) {
	if !strings.HasPrefix(value, SecretCipherPrefix) {
		return value, nil
	}

	sc, err := loadDefaultCipher()
	if err != nil {
		return "", err
	}

	if sc == nil {
		return "", errSecretKeyMissing
	}

	return sc.Decrypt(value)
}
