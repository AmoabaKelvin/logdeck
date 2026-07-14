package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/go-chi/chi/v5"
)

const (
	maxAPITokens       = 20
	maxAPITokenNameLen = 64
)

var (
	errTokenLimit     = fmt.Errorf("maximum of %d API tokens reached", maxAPITokens)
	errTokenNameTaken = errors.New("a token with this name already exists")
	errTokenNotFound  = errors.New("API token not found")
)

// lookupAPIToken resolves a presented API token against the tokens stored in
// the current file config. It reads through the config manager on every call
// so it stays correct across hot-swapped config updates. Comparison is
// constant-time over the SHA256 hashes.
func (ar *APIRouter) lookupAPIToken(token string) (string, string, bool) {
	hash := auth.HashAPIToken(token)
	fc := ar.manager.FileConfigSnapshot()

	name := ""
	scope := ""
	found := false
	for _, t := range fc.APITokens {
		if subtle.ConstantTimeCompare([]byte(hash), []byte(t.Hash)) == 1 {
			name = t.Name
			scope = t.Scope
			found = true
		}
	}
	return name, scope, found
}

// ListAPITokens handles GET /api/v1/settings/api-tokens.
func (ar *APIRouter) ListAPITokens(w http.ResponseWriter, r *http.Request) {
	fc := ar.manager.FileConfigSnapshot()
	tokens := make([]map[string]any, 0, len(fc.APITokens))
	for _, t := range fc.APITokens {
		tokens = append(tokens, map[string]any{
			"name":      t.Name,
			"prefix":    t.Prefix,
			"createdAt": t.CreatedAt,
			"scope":     auth.NormalizeAPITokenScope(t.Scope),
		})
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"tokens": tokens})
}

// CreateAPIToken handles POST /api/v1/settings/api-tokens. The full token is
// returned exactly once; only its hash is stored.
func (ar *APIRouter) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Scope string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if len(name) > maxAPITokenNameLen {
		http.Error(w, fmt.Sprintf("name must be at most %d characters", maxAPITokenNameLen), http.StatusBadRequest)
		return
	}

	scope := req.Scope
	if scope == "" {
		scope = auth.APITokenScopeAdmin
	}
	if scope != auth.APITokenScopeAdmin && scope != auth.APITokenScopeRead {
		http.Error(w, `scope must be "admin" or "read"`, http.StatusBadRequest)
		return
	}

	token, hash, prefix, err := auth.GenerateAPIToken()
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)
	err = ar.manager.UpdateAPITokens(func(current []config.APIToken) ([]config.APIToken, error) {
		if len(current) >= maxAPITokens {
			return nil, errTokenLimit
		}
		for _, t := range current {
			if t.Name == name {
				return nil, errTokenNameTaken
			}
		}
		return append(current, config.APIToken{
			Name:      name,
			Hash:      hash,
			Prefix:    prefix,
			CreatedAt: createdAt,
			Scope:     scope,
		}), nil
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errTokenLimit) || errors.Is(err, errTokenNameTaken) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	WriteJsonResponse(w, http.StatusCreated, map[string]any{
		"token":     token,
		"name":      name,
		"prefix":    prefix,
		"createdAt": createdAt,
		"scope":     scope,
	})
}

// DeleteAPIToken handles DELETE /api/v1/settings/api-tokens/{prefix}.
func (ar *APIRouter) DeleteAPIToken(w http.ResponseWriter, r *http.Request) {
	prefix := chi.URLParam(r, "prefix")

	err := ar.manager.UpdateAPITokens(func(current []config.APIToken) ([]config.APIToken, error) {
		next := current[:0]
		for _, t := range current {
			if t.Prefix != prefix {
				next = append(next, t)
			}
		}
		if len(next) == len(current) {
			return nil, errTokenNotFound
		}
		return next, nil
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errTokenNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "API token revoked"})
}
