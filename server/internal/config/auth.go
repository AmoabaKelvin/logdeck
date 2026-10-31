package config

import (
	"os"
	"strings"
	"time"
)

// User is a local account. Only a hash of its password is stored.
type User struct {
	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`
	// PasswordSalt is set only for SHA256 hashes written by older versions;
	// with it, PasswordHash is hex SHA256(password + salt) instead of bcrypt.
	PasswordSalt string `json:"passwordSalt,omitempty"`
	// Role is "admin" or "read", with the same meaning as API token scopes.
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// OIDCConfig configures sign-in through an OpenID Connect provider.
type OIDCConfig struct {
	Enabled      bool   `json:"enabled"`
	Issuer       string `json:"issuer,omitempty"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	RedirectURL  string `json:"redirectUrl,omitempty"`
	// GroupsClaim names the ID token claim that lists the user's groups;
	// "groups" when empty.
	GroupsClaim string   `json:"groupsClaim,omitempty"`
	AdminGroups []string `json:"adminGroups,omitempty"`
	ReadGroups  []string `json:"readGroups,omitempty"`
	// DefaultRole is given to users in none of the groups above. Empty refuses
	// them.
	DefaultRole string `json:"defaultRole,omitempty"`
}

// migrateLegacyAdmin makes the single admin of older versions the first user,
// so existing settings-configured logins keep working with zero user action.
// Once there are users, the legacy fields only mirror one of them (see
// mirrorLegacyAdmin) and are not read again.
func migrateLegacyAdmin(a *FileAuthConfig) bool {
	if len(a.Users) > 0 || a.AdminUsername == "" || a.AdminPasswordHash == "" {
		return false
	}
	a.Users = []User{{
		Username:     a.AdminUsername,
		PasswordHash: a.AdminPasswordHash,
		PasswordSalt: a.AdminPasswordSalt,
		Role:         "admin",
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}}
	return true
}

// mirrorLegacyAdmin copies the first admin user into the single-admin fields
// older versions read, so rolling back to one keeps requiring a login rather
// than finding no admin and running open.
func mirrorLegacyAdmin(a *FileAuthConfig) {
	a.AdminUsername, a.AdminPasswordHash, a.AdminPasswordSalt = "", "", ""
	for _, u := range a.Users {
		if u.Role == "admin" {
			a.AdminUsername, a.AdminPasswordHash, a.AdminPasswordSalt = u.Username, u.PasswordHash, u.PasswordSalt
			return
		}
	}
}

// oidcFromEnv reads OIDC settings from OIDC_* env vars. It returns nil when
// OIDC_ISSUER is unset, leaving OIDC to the config file.
func oidcFromEnv() *OIDCConfig {
	issuer := os.Getenv("OIDC_ISSUER")
	if issuer == "" {
		return nil
	}
	return &OIDCConfig{
		Enabled:      true,
		Issuer:       issuer,
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("OIDC_REDIRECT_URL"),
		GroupsClaim:  os.Getenv("OIDC_GROUPS_CLAIM"),
		AdminGroups:  splitList(os.Getenv("OIDC_ADMIN_GROUPS")),
		ReadGroups:   splitList(os.Getenv("OIDC_READ_GROUPS")),
		DefaultRole:  os.Getenv("OIDC_DEFAULT_ROLE"),
	}
}

// splitList splits a comma-separated list, dropping empty entries.
func splitList(s string) []string {
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// OIDC returns the OIDC settings in effect, from OIDC_* env vars when
// OIDC_ISSUER is set and from the config file otherwise, or nil when OIDC is
// not configured.
func (m *Manager) OIDC() *OIDCConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.envOIDC != nil {
		return m.envOIDC
	}
	if m.fileConfig.Auth != nil && m.fileConfig.Auth.OIDC != nil {
		oidc := *m.fileConfig.Auth.OIDC
		return &oidc
	}
	return nil
}

// UpdateSignIn applies a mutation to the users and OIDC settings in the auth
// section atomically. Unlike UpdateAuth it is allowed when env vars configure
// auth: users and OIDC live only in the file, next to an env-defined admin.
func (m *Manager) UpdateSignIn(mutate func(current *FileAuthConfig) (*FileAuthConfig, error)) error {
	m.mu.Lock()
	return m.updateAuthLocked(mutate)
}
