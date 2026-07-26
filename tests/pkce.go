package tests

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

func createPKCE() (verifier string, challenge string, err error) {
	randomBytes := make([]byte, 64)

	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", fmt.Errorf("generate PKCE verifier: %w", err)
	}

	verifier = base64.RawURLEncoding.EncodeToString(randomBytes)

	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])

	return verifier, challenge, nil
}
