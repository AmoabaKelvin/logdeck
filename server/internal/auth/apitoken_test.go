package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIToken(t *testing.T) {
	token, hash, prefix, err := GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken failed: %v", err)
	}

	if !strings.HasPrefix(token, APITokenPrefix) {
		t.Errorf("token %q does not start with %q", token, APITokenPrefix)
	}
	// ldk_ (4) + base64url of 32 bytes without padding (43)
	if len(token) != 47 {
		t.Errorf("expected token length 47, got %d", len(token))
	}
	if prefix != token[:APITokenDisplayLen] {
		t.Errorf("prefix %q does not match first %d chars of token", prefix, APITokenDisplayLen)
	}
	if hash != HashAPIToken(token) {
		t.Errorf("returned hash does not match HashAPIToken(token)")
	}
	// sha256 hex is 64 chars
	if len(hash) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars", len(hash))
	}
}

func TestNormalizeAPITokenScope(t *testing.T) {
	cases := map[string]string{
		"":                 APITokenScopeAdmin, // legacy tokens without a scope are admin
		APITokenScopeAdmin: APITokenScopeAdmin,
		APITokenScopeRead:  APITokenScopeRead,
	}
	for in, want := range cases {
		if got := NormalizeAPITokenScope(in); got != want {
			t.Errorf("NormalizeAPITokenScope(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenerateAPITokenUnique(t *testing.T) {
	a, _, _, err := GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken failed: %v", err)
	}
	b, _, _, err := GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken failed: %v", err)
	}
	if a == b {
		t.Error("two generated tokens are identical")
	}
}
