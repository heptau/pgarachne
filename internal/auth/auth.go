package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashToken hashes the raw token using SHA-256.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// TODO: Helper functions for JWT generation/validation can be moved here too.
