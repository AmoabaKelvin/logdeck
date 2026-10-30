package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/go-chi/chi/v5"
)

const minPasswordLen = 8

var (
	usernameRegex = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@-]{0,63}$`)

	errUserExists   = errors.New("a user with this name already exists")
	errUserNotFound = errors.New("user not found")
	errLastAdmin    = errors.New("at least one admin must remain")
	errEnvUser      = errors.New("this user is defined via environment variable and cannot be changed from the UI")

	errFirstAdminRequired    = errors.New("adminUsername and newPassword are required when first enabling auth")
	errAdminPasswordRequired = errors.New("adminUsername is not a user; send newPassword to add it as an admin, or manage users under /settings/users")
)

// userErrorStatus maps user management errors to HTTP status codes.
func userErrorStatus(err error) int {
	switch {
	case errors.Is(err, errUserNotFound):
		return http.StatusNotFound
	case errors.Is(err, errFirstAdminRequired), errors.Is(err, errAdminPasswordRequired):
		return http.StatusBadRequest
	case errors.Is(err, errUserExists), errors.Is(err, errLastAdmin), errors.Is(err, errEnvUser):
		return http.StatusConflict
	}
	return settingsErrorStatus(err)
}

// usernameParam reads the {username} path parameter. Chi matches on the raw
// path, so an escaped character such as the %40 of an email arrives escaped.
func usernameParam(r *http.Request) string {
	raw := chi.URLParam(r, "username")
	if name, err := url.PathUnescape(raw); err == nil {
		return name
	}
	return raw
}

// envAdmin returns the admin set by ADMIN_USERNAME, or "".
func (ar *APIRouter) envAdmin() string {
	if svc := ar.registry.Auth(); svc != nil {
		return svc.EnvAdmin()
	}
	return ""
}

// keepsAnAdmin reports whether someone could still administer LogDeck with
// these users: one of them is an admin, or an env-defined admin exists.
func (ar *APIRouter) keepsAnAdmin(users []config.User) bool {
	return ar.envAdmin() != "" || slices.ContainsFunc(users, func(u config.User) bool { return u.Role == auth.RoleAdmin })
}

// validatePassword checks a new password before it is hashed. Bcrypt reads at
// most 72 bytes, so a longer password is refused rather than truncated.
func validatePassword(password string) error {
	if len(password) < minPasswordLen {
		return fmt.Errorf("password must be at least %d characters", minPasswordLen)
	}
	if len(password) > 72 {
		return errors.New("password must be at most 72 bytes")
	}
	return nil
}

// ListUsers handles GET /api/v1/settings/users.
func (ar *APIRouter) ListUsers(w http.ResponseWriter, r *http.Request) {
	users := []map[string]any{}
	envAdmin := ar.envAdmin()
	if envAdmin != "" {
		users = append(users, map[string]any{"username": envAdmin, "role": auth.RoleAdmin, "source": config.SourceEnv})
	}
	if fc := ar.manager.FileConfigSnapshot(); fc.Auth != nil {
		for _, u := range fc.Auth.Users {
			if u.Username == envAdmin {
				continue
			}
			role := auth.RoleRead
			if u.Role == auth.RoleAdmin {
				role = auth.RoleAdmin
			}
			users = append(users, map[string]any{"username": u.Username, "role": role, "source": config.SourceFile, "createdAt": u.CreatedAt})
		}
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"users": users})
}

// CreateUser handles POST /api/v1/settings/users.
func (ar *APIRouter) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !usernameRegex.MatchString(req.Username) {
		http.Error(w, "username must be 1-64 letters, digits, or . _ @ -, starting with a letter or digit", http.StatusBadRequest)
		return
	}
	if req.Role != auth.RoleAdmin && req.Role != auth.RoleRead {
		http.Error(w, `role must be "admin" or "read"`, http.StatusBadRequest)
		return
	}
	if err := validatePassword(req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Username == ar.envAdmin() {
		http.Error(w, errUserExists.Error(), http.StatusConflict)
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	err = ar.manager.UpdateSignIn(func(authCfg *config.FileAuthConfig) (*config.FileAuthConfig, error) {
		if slices.ContainsFunc(authCfg.Users, func(u config.User) bool { return u.Username == req.Username }) {
			return nil, errUserExists
		}
		authCfg.Users = append(authCfg.Users, config.User{
			Username:     req.Username,
			PasswordHash: hash,
			Role:         req.Role,
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		})
		return authCfg, nil
	})
	if err != nil {
		http.Error(w, err.Error(), userErrorStatus(err))
		return
	}
	WriteJsonResponse(w, http.StatusCreated, map[string]any{"message": "User created"})
}

// UpdateUser handles PUT /api/v1/settings/users/{username}: it changes the
// role, resets the password, or both. Either takes effect on the user's next
// request; a reset password also ends their existing sessions.
func (ar *APIRouter) UpdateUser(w http.ResponseWriter, r *http.Request) {
	username := usernameParam(r)
	var req struct {
		Role     string `json:"role,omitempty"`
		Password string `json:"password,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Role != "" && req.Role != auth.RoleAdmin && req.Role != auth.RoleRead {
		http.Error(w, `role must be "admin" or "read"`, http.StatusBadRequest)
		return
	}
	var hash string
	if req.Password != "" {
		if err := validatePassword(req.Password); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var err error
		if hash, err = auth.HashPassword(req.Password); err != nil {
			http.Error(w, "failed to hash password", http.StatusInternalServerError)
			return
		}
	}
	if username == ar.envAdmin() {
		http.Error(w, errEnvUser.Error(), http.StatusConflict)
		return
	}

	err := ar.manager.UpdateSignIn(func(authCfg *config.FileAuthConfig) (*config.FileAuthConfig, error) {
		i := slices.IndexFunc(authCfg.Users, func(u config.User) bool { return u.Username == username })
		if i < 0 {
			return nil, errUserNotFound
		}
		if req.Role != "" {
			authCfg.Users[i].Role = req.Role
		}
		if hash != "" {
			authCfg.Users[i].PasswordHash = hash
			authCfg.Users[i].PasswordSalt = ""
		}
		if !ar.keepsAnAdmin(authCfg.Users) {
			return nil, errLastAdmin
		}
		return authCfg, nil
	})
	if err != nil {
		http.Error(w, err.Error(), userErrorStatus(err))
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "User updated"})
}

// DeleteUser handles DELETE /api/v1/settings/users/{username}. The user's
// sessions stop working on their next request.
func (ar *APIRouter) DeleteUser(w http.ResponseWriter, r *http.Request) {
	username := usernameParam(r)
	if username == ar.envAdmin() {
		http.Error(w, errEnvUser.Error(), http.StatusConflict)
		return
	}

	err := ar.manager.UpdateSignIn(func(authCfg *config.FileAuthConfig) (*config.FileAuthConfig, error) {
		i := slices.IndexFunc(authCfg.Users, func(u config.User) bool { return u.Username == username })
		if i < 0 {
			return nil, errUserNotFound
		}
		authCfg.Users = slices.Delete(authCfg.Users, i, i+1)
		if !ar.keepsAnAdmin(authCfg.Users) {
			return nil, errLastAdmin
		}
		return authCfg, nil
	})
	if err != nil {
		http.Error(w, err.Error(), userErrorStatus(err))
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "User deleted"})
}

// oidcSettings is the OIDC section of GET /settings, with the client secret
// masked.
func oidcSettings(cfg *config.OIDCConfig, source config.Source) map[string]any {
	resp := map[string]any{"source": source, "enabled": false}
	if cfg == nil {
		return resp
	}
	secret := ""
	if cfg.ClientSecret != "" {
		secret = secretMask
	}
	resp["enabled"] = cfg.Enabled
	resp["issuer"] = cfg.Issuer
	resp["clientId"] = cfg.ClientID
	resp["clientSecret"] = secret
	resp["redirectUrl"] = cfg.RedirectURL
	resp["groupsClaim"] = cfg.GroupsClaim
	resp["adminGroups"] = nonNil(cfg.AdminGroups)
	resp["readGroups"] = nonNil(cfg.ReadGroups)
	resp["defaultRole"] = cfg.DefaultRole
	return resp
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// UpdateOIDC handles PUT /api/v1/settings/oidc. A changed configuration ends
// the existing OIDC sessions.
func (ar *APIRouter) UpdateOIDC(w http.ResponseWriter, r *http.Request) {
	var req config.OIDCConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if ar.manager.Sources().OIDC == config.SourceEnv {
		http.Error(w, "OIDC is configured via environment variables and cannot be changed from the UI", http.StatusConflict)
		return
	}

	req.Issuer = strings.TrimRight(strings.TrimSpace(req.Issuer), "/")
	req.ClientID = strings.TrimSpace(req.ClientID)
	req.RedirectURL = strings.TrimSpace(req.RedirectURL)
	req.GroupsClaim = strings.TrimSpace(req.GroupsClaim)
	req.AdminGroups = trimList(req.AdminGroups)
	req.ReadGroups = trimList(req.ReadGroups)
	if req.Enabled {
		if !isHTTPURL(req.Issuer) {
			http.Error(w, "issuer must be an http:// or https:// URL", http.StatusBadRequest)
			return
		}
		if req.ClientID == "" {
			http.Error(w, "clientId is required", http.StatusBadRequest)
			return
		}
		if !isHTTPURL(req.RedirectURL) {
			http.Error(w, "redirectUrl must be an http:// or https:// URL", http.StatusBadRequest)
			return
		}
	}
	if req.DefaultRole != "" && req.DefaultRole != auth.RoleAdmin && req.DefaultRole != auth.RoleRead {
		http.Error(w, `defaultRole must be "", "admin" or "read"`, http.StatusBadRequest)
		return
	}

	err := ar.manager.UpdateSignIn(func(authCfg *config.FileAuthConfig) (*config.FileAuthConfig, error) {
		if req.ClientSecret == secretMask {
			req.ClientSecret = ""
			if authCfg.OIDC != nil {
				req.ClientSecret = authCfg.OIDC.ClientSecret
			}
		}
		authCfg.OIDC = &req
		return authCfg, nil
	})
	if err != nil {
		http.Error(w, err.Error(), settingsErrorStatus(err))
		return
	}
	WriteJsonResponse(w, http.StatusOK, map[string]any{"message": "OIDC settings updated"})
}

func trimList(list []string) []string {
	var out []string
	for _, s := range list {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func isHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
