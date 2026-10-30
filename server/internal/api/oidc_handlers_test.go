package api

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
	"github.com/AmoabaKelvin/logdeck/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

// fakeOIDCProvider is a minimal OpenID Connect provider: discovery, JWKS, and
// a token endpoint that checks the PKCE verifier against the challenge the
// authorization request carried.
type fakeOIDCProvider struct {
	srv *httptest.Server
	key *rsa.PrivateKey

	mu    sync.Mutex
	codes map[string]fakeGrant
}

type fakeGrant struct {
	challenge string
	nonce     string
	claims    jwt.MapClaims
}

func newFakeOIDCProvider(t *testing.T) *fakeOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &fakeOIDCProvider{key: key, codes: map[string]fakeGrant{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                p.srv.URL,
			"authorization_endpoint":                p.srv.URL + "/authorize",
			"token_endpoint":                        p.srv.URL + "/token",
			"jwks_uri":                              p.srv.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
			"scopes_supported":                      []string{"openid", "profile", "email", "groups"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		b64 := base64.RawURLEncoding.EncodeToString
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
			"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		p.mu.Lock()
		grant, ok := p.codes[r.Form.Get("code")]
		delete(p.codes, r.Form.Get("code"))
		p.mu.Unlock()
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != grant.challenge {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		claims := jwt.MapClaims{
			"iss": p.srv.URL, "aud": "logdeck", "sub": "user-1", "nonce": grant.nonce,
			"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		}
		for k, v := range grant.claims {
			claims[k] = v
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = "k1"
		idToken, err := tok.SignedString(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "id_token": idToken})
	})
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

// authorize plays the user signing in at the provider: it checks the
// authorization request and returns the code the provider would send back.
func (p *fakeOIDCProvider) authorize(t *testing.T, authURL string, claims jwt.MapClaims) (state, code string) {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil || !strings.HasPrefix(authURL, p.srv.URL+"/authorize") {
		t.Fatalf("redirected to %q, want the provider", authURL)
	}
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("nonce") == "" || q.Get("state") == "" {
		t.Fatalf("authorization request lacks PKCE, nonce or state: %s", u.RawQuery)
	}
	if !strings.Contains(q.Get("scope"), "groups") {
		t.Errorf("scope %q should ask for groups the provider offers", q.Get("scope"))
	}
	code = rand.Text()
	p.mu.Lock()
	p.codes[code] = fakeGrant{challenge: q.Get("code_challenge"), nonce: q.Get("nonce"), claims: claims}
	p.mu.Unlock()
	return q.Get("state"), code
}

var oidcTestIP atomic.Int32

// oidcRequest sends a request from its own client IP, so the tests stay
// clear of the shared login rate limit.
func oidcRequest(router http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	n := oidcTestIP.Add(1)
	r.RemoteAddr = fmt.Sprintf("10.188.%d.%d:1000", n/250, n%250)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	router.ServeHTTP(w, r)
	return w
}

func newOIDCTestRouter(t *testing.T, p *fakeOIDCProvider) (http.Handler, *auth.Service) {
	t.Helper()
	clearEnvForAuthTest(t)
	svc, err := auth.NewService(&config.FileAuthConfig{Enabled: true, JWTSecret: "test-secret"}, &config.OIDCConfig{
		Enabled:      true,
		Issuer:       p.srv.URL,
		ClientID:     "logdeck",
		ClientSecret: "client-secret",
		RedirectURL:  "http://logdeck.test/api/v1/auth/oidc/callback",
		AdminGroups:  []string{"ops"},
		ReadGroups:   []string{"dev"},
	})
	if err != nil || svc == nil {
		t.Fatalf("NewService = %v, %v", svc, err)
	}
	return newTestRouter(t, svc), svc
}

// startOIDC begins a sign-in and returns the provider URL and flow cookie.
func startOIDC(t *testing.T, router http.Handler) (string, *http.Cookie) {
	t.Helper()
	w := oidcRequest(router, "/api/v1/auth/oidc/login?redirect=/containers/abc&check=page-check", nil)
	if w.Code != http.StatusFound {
		t.Fatalf("login: expected 302, got %d: %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != oidcFlowCookie || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected one HttpOnly, SameSite=Lax flow cookie, got %+v", cookies)
	}
	return w.Header().Get("Location"), cookies[0]
}

func TestOIDCSignInMapsGroupsToRoles(t *testing.T) {
	p := newFakeOIDCProvider(t)
	router, _ := newOIDCTestRouter(t, p)

	for _, tc := range []struct {
		groups   any
		wantRole string
	}{
		{[]string{"staff", "ops"}, auth.RoleAdmin},
		{[]string{"dev"}, auth.RoleRead},
		{"dev", auth.RoleRead}, // a single group sent as a string
	} {
		authURL, cookie := startOIDC(t, router)
		state, code := p.authorize(t, authURL, jwt.MapClaims{"preferred_username": "alice", "groups": tc.groups})

		w := oidcRequest(router, "/api/v1/auth/oidc/callback?state="+state+"&code="+code, cookie)
		if w.Code != http.StatusFound {
			t.Fatalf("callback: expected 302, got %d: %s", w.Code, w.Body.String())
		}
		loc := w.Header().Get("Location")
		prefix := "/login?redirect=" + url.QueryEscape("/containers/abc") + "#"
		if !strings.HasPrefix(loc, prefix) {
			t.Fatalf("callback redirected to %q, want %q...", loc, prefix)
		}
		fragment, _ := url.ParseQuery(strings.TrimPrefix(loc, prefix))
		token := fragment.Get("oidc_token")
		if fragment.Get("check") != "page-check" {
			t.Errorf("fragment %q should carry back the check the page sent", loc)
		}

		me := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(me, r)
		var body struct {
			User struct{ Username, Role string }
		}
		_ = json.Unmarshal(me.Body.Bytes(), &body)
		if me.Code != http.StatusOK || body.User.Username != "alice" || body.User.Role != tc.wantRole {
			t.Errorf("groups %v: /auth/me = %d %s, want alice as %s", tc.groups, me.Code, me.Body.String(), tc.wantRole)
		}
	}
}

func TestOIDCSignInRefusals(t *testing.T) {
	p := newFakeOIDCProvider(t)
	router, _ := newOIDCTestRouter(t, p)

	callback := func(state, code string, cookie *http.Cookie) string {
		w := oidcRequest(router, "/api/v1/auth/oidc/callback?state="+url.QueryEscape(state)+"&code="+url.QueryEscape(code), cookie)
		return w.Header().Get("Location")
	}

	// No mapped group and no default role.
	authURL, cookie := startOIDC(t, router)
	state, code := p.authorize(t, authURL, jwt.MapClaims{"groups": []string{"guests"}})
	if loc := callback(state, code, cookie); loc != "/login?oidc_error=denied" {
		t.Errorf("unmapped user: redirected to %q", loc)
	}

	// The callback without the cookie of the browser that started the flow.
	authURL, _ = startOIDC(t, router)
	state, code = p.authorize(t, authURL, jwt.MapClaims{"groups": []string{"ops"}})
	if loc := callback(state, code, nil); loc != "/login?oidc_error=expired" {
		t.Errorf("missing flow cookie: redirected to %q", loc)
	}

	// Another sign-in's cookie: the state does not match.
	authURL, _ = startOIDC(t, router)
	_, otherCookie := startOIDC(t, router)
	state, code = p.authorize(t, authURL, jwt.MapClaims{"groups": []string{"ops"}})
	if loc := callback(state, code, otherCookie); loc != "/login?oidc_error=failed" {
		t.Errorf("mismatched state: redirected to %q", loc)
	}

	// A code issued for another sign-in's PKCE challenge.
	authURL1, _ := startOIDC(t, router)
	authURL2, cookie2 := startOIDC(t, router)
	_, code1 := p.authorize(t, authURL1, jwt.MapClaims{"groups": []string{"ops"}})
	state2, _ := p.authorize(t, authURL2, jwt.MapClaims{"groups": []string{"ops"}})
	if loc := callback(state2, code1, cookie2); loc != "/login?oidc_error=failed" {
		t.Errorf("wrong PKCE verifier: redirected to %q", loc)
	}

	// An ID token whose nonce belongs to another sign-in.
	authURL, cookie = startOIDC(t, router)
	state, code = p.authorize(t, authURL, jwt.MapClaims{"groups": []string{"ops"}, "nonce": "replayed"})
	if loc := callback(state, code, cookie); loc != "/login?oidc_error=failed" {
		t.Errorf("wrong nonce: redirected to %q", loc)
	}
}

// Changing or turning off OIDC ends the sessions it issued.
func TestOIDCSessionEndsWhenSettingsChange(t *testing.T) {
	p := newFakeOIDCProvider(t)
	router, svc := newOIDCTestRouter(t, p)
	authURL, cookie := startOIDC(t, router)
	state, code := p.authorize(t, authURL, jwt.MapClaims{"preferred_username": "alice", "groups": []string{"dev"}})
	loc := oidcRequest(router, "/api/v1/auth/oidc/callback?state="+state+"&code="+code, cookie).Header().Get("Location")
	fragment, _ := url.ParseQuery(loc[strings.Index(loc, "#")+1:])
	token := fragment.Get("oidc_token")

	claims, err := svc.VerifyToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.SessionUser(claims); !ok {
		t.Fatal("fresh OIDC session should count")
	}

	fileAuth := &config.FileAuthConfig{Enabled: true, JWTSecret: "test-secret"}
	remapped, _ := auth.NewService(fileAuth, &config.OIDCConfig{
		Enabled: true, Issuer: p.srv.URL, ClientID: "logdeck", RedirectURL: "http://logdeck.test/cb",
		AdminGroups: []string{"ops", "dev"},
	})
	if _, ok := remapped.SessionUser(claims); ok {
		t.Error("a new group mapping must end the old OIDC sessions")
	}
	disabled, _ := auth.NewService(fileAuth, nil)
	if _, ok := disabled.SessionUser(claims); ok {
		t.Error("turning OIDC off must end its sessions")
	}
}
