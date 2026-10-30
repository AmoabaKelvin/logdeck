package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

var (
	ErrOIDCDisabled = errors.New("OIDC sign-in is not enabled")
	// ErrOIDCDenied means the provider signed the user in but none of their
	// groups maps to a role and there is no default role.
	ErrOIDCDenied = errors.New("this account is not allowed to sign in to LogDeck")
)

// oidcFlowTTL bounds how long a user has to finish signing in at the provider.
const oidcFlowTTL = 10 * time.Minute

// oidcClient signs users in through an OpenID Connect provider with the
// authorization code flow, PKCE, state and nonce.
type oidcClient struct {
	cfg   config.OIDCConfig
	stamp string
	// flowKey signs the flow cookie. It is derived apart from the session key
	// so a flow token can never pass for a session.
	flowKey []byte

	mu       sync.Mutex
	provider *oidc.Provider // discovered on the first sign-in, then reused
	scopes   []string
}

// oidcFlow is what the browser carries from the start of a sign-in to the
// callback.
type oidcFlow struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
	Redirect string `json:"redirect,omitempty"`
	jwt.RegisteredClaims
}

func newOIDCClient(cfg config.OIDCConfig, s *Service) *oidcClient {
	if cfg.GroupsClaim == "" {
		cfg.GroupsClaim = "groups"
	}
	mac := hmac.New(sha256.New, s.jwtSecret)
	mac.Write([]byte("oidc-flow"))
	return &oidcClient{
		cfg: cfg,
		// Changing who is let in, or as what, ends the existing OIDC sessions.
		stamp: s.stamp("oidc", cfg.Issuer, cfg.ClientID, cfg.GroupsClaim,
			strings.Join(cfg.AdminGroups, ","), strings.Join(cfg.ReadGroups, ","), cfg.DefaultRole),
		flowKey: mac.Sum(nil),
	}
}

// OIDCEnabled reports whether users can sign in through an OIDC provider.
func (s *Service) OIDCEnabled() bool {
	return s.oidc != nil
}

// oauthConfig discovers the provider on first use. Discovery is a network
// call, so it happens at sign-in, never per request or at startup.
func (c *oidcClient) oauthConfig(ctx context.Context) (*oauth2.Config, *oidc.Provider, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.provider == nil {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		provider, err := oidc.NewProvider(ctx, c.cfg.Issuer)
		if err != nil {
			return nil, nil, fmt.Errorf("OIDC discovery failed: %w", err)
		}
		// Ask for the groups scope only where the provider offers it: some,
		// Google among them, reject a scope they don't know.
		var discovery struct {
			ScopesSupported []string `json:"scopes_supported"`
		}
		_ = provider.Claims(&discovery)
		c.scopes = []string{oidc.ScopeOpenID, "profile", "email"}
		if slices.Contains(discovery.ScopesSupported, "groups") {
			c.scopes = append(c.scopes, "groups")
		}
		c.provider = provider
	}
	return &oauth2.Config{
		ClientID:     c.cfg.ClientID,
		ClientSecret: c.cfg.ClientSecret,
		RedirectURL:  c.cfg.RedirectURL,
		Endpoint:     c.provider.Endpoint(),
		Scopes:       c.scopes,
	}, c.provider, nil
}

// OIDCStart begins a sign-in. It returns the provider URL to send the browser
// to and a flow value for a cookie. The callback must present the flow, which
// binds the sign-in to the browser that started it. redirect is the page to
// return to afterwards.
func (s *Service) OIDCStart(ctx context.Context, redirect string) (authURL, flow string, err error) {
	if s.oidc == nil {
		return "", "", ErrOIDCDisabled
	}
	oauthCfg, _, err := s.oidc.oauthConfig(ctx)
	if err != nil {
		return "", "", err
	}

	f := oidcFlow{
		State:    rand.Text(),
		Nonce:    rand.Text(),
		Verifier: oauth2.GenerateVerifier(),
		Redirect: redirect,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(oidcFlowTTL)),
		},
	}
	flow, err = jwt.NewWithClaims(jwt.SigningMethodHS256, &f).SignedString(s.oidc.flowKey)
	if err != nil {
		return "", "", err
	}
	authURL = oauthCfg.AuthCodeURL(f.State, oidc.Nonce(f.Nonce), oauth2.S256ChallengeOption(f.Verifier))
	return authURL, flow, nil
}

// OIDCFinish completes a sign-in on the provider's callback. It checks the
// state against the flow, exchanges the code with the PKCE verifier, verifies
// the ID token and its nonce, and maps the user's groups to a role. It
// returns a session token and the page to return to.
func (s *Service) OIDCFinish(ctx context.Context, flow, state, code string) (token, redirect string, err error) {
	if s.oidc == nil {
		return "", "", ErrOIDCDisabled
	}
	var f oidcFlow
	_, err = jwt.ParseWithClaims(flow, &f, func(t *jwt.Token) (any, error) {
		return s.oidc.flowKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(f.State)) != 1 {
		return "", "", errors.New("OIDC sign-in expired or was started in another browser")
	}

	oauthCfg, provider, err := s.oidc.oauthConfig(ctx)
	if err != nil {
		return "", "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	oauthToken, err := oauthCfg.Exchange(ctx, code, oauth2.VerifierOption(f.Verifier))
	if err != nil {
		return "", "", fmt.Errorf("OIDC code exchange failed: %w", err)
	}
	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok {
		return "", "", errors.New("OIDC provider returned no ID token")
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: s.oidc.cfg.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return "", "", fmt.Errorf("OIDC ID token is invalid: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(f.Nonce)) != 1 {
		return "", "", errors.New("OIDC ID token nonce does not match")
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return "", "", fmt.Errorf("OIDC ID token claims: %w", err)
	}
	username := oidcUsername(claims, idToken.Subject)
	role := s.oidc.role(stringList(claims[s.oidc.cfg.GroupsClaim]))
	if role == "" {
		return "", "", ErrOIDCDenied
	}

	token, err = s.signSession(Claims{Username: username, Role: role, Stamp: s.oidc.stamp, OIDC: true})
	if err != nil {
		return "", "", err
	}
	return token, f.Redirect, nil
}

// oidcUsername picks the name a user is shown and audited as: their preferred
// username, else their email, else the provider's subject id.
func oidcUsername(claims map[string]any, subject string) string {
	for _, key := range []string{"preferred_username", "email"} {
		if v, ok := claims[key].(string); ok && v != "" {
			return v
		}
	}
	return subject
}

// role maps a user's groups to a role: admin groups first, then read groups,
// then the default role, which may be "" to refuse the user.
func (c *oidcClient) role(groups []string) string {
	inAny := func(names []string) bool {
		return slices.ContainsFunc(groups, func(g string) bool { return slices.Contains(names, g) })
	}
	switch {
	case inAny(c.cfg.AdminGroups):
		return RoleAdmin
	case inAny(c.cfg.ReadGroups):
		return RoleRead
	case c.cfg.DefaultRole == RoleAdmin, c.cfg.DefaultRole == RoleRead:
		return c.cfg.DefaultRole
	}
	return ""
}

// stringList reads a groups claim, which providers send as a list of strings
// or, with a single group, as one string.
func stringList(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
