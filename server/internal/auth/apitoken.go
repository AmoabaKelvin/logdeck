package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// APITokenPrefix is the fixed prefix of every LogDeck API token.
const APITokenPrefix = "ldk_"

// APITokenDisplayLen is how many leading characters of a token are stored
// for display and identification (includes the "ldk_" prefix).
const APITokenDisplayLen = 12

// API token scopes. Admin tokens have full access; read tokens can only
// perform read operations.
const (
	APITokenScopeAdmin = "admin"
	APITokenScopeRead  = "read"
)

// NormalizeAPITokenScope maps a stored scope to its effective value. Tokens
// created before scopes existed have an empty scope and are treated as admin.
// Any other unrecognized value (e.g. a hand-edited config) fails closed to
// read so a typo never silently grants admin access.
func NormalizeAPITokenScope(scope string) string {
	switch scope {
	case "", APITokenScopeAdmin:
		return APITokenScopeAdmin
	default:
		return APITokenScopeRead
	}
}

// GenerateAPIToken creates a new API token from 32 random bytes.
// It returns the full token (shown to the user exactly once), the SHA256 hex
// hash to persist, and the display prefix used to identify the token.
func GenerateAPIToken() (token, hash, prefix string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", err
	}
	token = APITokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	return token, HashAPIToken(token), token[:APITokenDisplayLen], nil
}

// HashAPIToken returns the hex-encoded SHA256 hash of a token. API tokens are
// high-entropy secrets, so a fast unsalted hash is appropriate for
// per-request lookup.
func HashAPIToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
