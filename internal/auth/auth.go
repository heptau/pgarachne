// Package auth centralises credential primitives for PgArachne: JWT issue
// and parse (used for short-lived session tokens) and the SHA-256 hashing
// helper used by external token-management tooling.
package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashToken hashes a raw token using SHA-256. The same algorithm is applied
// by the pgarachne.verify_api_token SQL function when looking up long-lived
// API tokens, so values produced here are interchangeable with values stored
// server-side.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
