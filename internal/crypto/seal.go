// Package crypto provides AES-GCM sealing for sensitive CRM secrets at rest.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const sealedPrefix = "enc1:"

// DeriveKey returns a 32-byte AES key from a secret string.
func DeriveKey(secret string) []byte {
	s := strings.TrimSpace(secret)
	if len(s) < 16 {
		return nil
	}
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

// IsSealed reports whether a stored value was sealed by this package.
func IsSealed(value string) bool {
	return strings.HasPrefix(value, sealedPrefix)
}

// Seal encrypts plaintext with AES-256-GCM. Returns empty string for empty input.
func Seal(key []byte, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if len(key) != 32 {
		return "", errors.New("crypto: invalid key length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return sealedPrefix + base64.RawStdEncoding.EncodeToString(ciphertext), nil
}

// Unseal decrypts a value produced by Seal.
func Unseal(key []byte, sealed string) (string, error) {
	if !IsSealed(sealed) {
		return sealed, nil
	}
	if len(key) != 32 {
		return "", errors.New("crypto: invalid key length")
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(sealed, sealedPrefix))
	if err != nil {
		return "", fmt.Errorf("crypto: decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("crypto: ciphertext too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decrypt: %w", err)
	}
	return string(plain), nil
}
