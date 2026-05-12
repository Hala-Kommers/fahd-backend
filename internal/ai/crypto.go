package ai

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

func deriveKey(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

func EncryptAPIKey(plaintext, jwtSecret string) (string, error) {
	key := deriveKey(jwtSecret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("aes gcm: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptAPIKey(encoded, jwtSecret string) (string, error) {
	key := deriveKey(jwtSecret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes new cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("aes gcm: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}
