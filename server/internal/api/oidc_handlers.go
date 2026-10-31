package api

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/AmoabaKelvin/logdeck/internal/auth"
)

// oidcFlowCookie carries a sign-in from its start to the provider's callback.
const (
	oidcFlowCookie     = "logdeck_oidc"
	oidcFlowCookiePath = "/api/v1/auth/oidc"
)

// handleOIDCLogin handles GET /api/v1/auth/oidc/login: it sends the browser to
// the OIDC provider. Public, no auth.
func (ar *APIRouter) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	svc := ar.registry.Auth()
	if svc == nil || !svc.OIDCEnabled() {
		http.Error(w, "OIDC sign-in is not enabled", http.StatusNotFound)
		return
	}

	// check is a random value the login page keeps in its own storage and
	// gets back with the token. A token in a link someone else sent comes
	// without it, so the page can refuse to sign in as somebody else.
	q := r.URL.Query()
	check := q.Get("check")
	if len(check) > 128 {
		http.Error(w, "check is too long", http.StatusBadRequest)
		return
	}
	authURL, flow, err := svc.OIDCStart(r.Context(), auth.OIDCReturn{Redirect: sameSitePath(q.Get("redirect")), Check: check})
	if err != nil {
		log.Printf("OIDC sign-in could not start: %v", err)
		redirectToLogin(w, r, "failed")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oidcFlowCookie,
		Value:    flow,
		Path:     oidcFlowCookiePath,
		MaxAge:   600,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		// Lax, not Strict: the provider's redirect back is a cross-site
		// top-level navigation, which must carry the cookie.
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleOIDCCallback handles GET /api/v1/auth/oidc/callback, where the
// provider sends the browser back. The session token goes to the login page
// in the URL fragment, which browsers never send to a server or in a Referer.
func (ar *APIRouter) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	// The flow is single use either way.
	http.SetCookie(w, &http.Cookie{Name: oidcFlowCookie, Path: oidcFlowCookiePath, MaxAge: -1, HttpOnly: true, Secure: isHTTPS(r)})

	svc := ar.registry.Auth()
	if svc == nil || !svc.OIDCEnabled() {
		http.Error(w, "OIDC sign-in is not enabled", http.StatusNotFound)
		return
	}

	q := r.URL.Query()
	if q.Get("error") != "" {
		redirectToLogin(w, r, "cancelled")
		return
	}
	cookie, err := r.Cookie(oidcFlowCookie)
	if err != nil {
		redirectToLogin(w, r, "expired")
		return
	}

	token, ret, err := svc.OIDCFinish(r.Context(), cookie.Value, q.Get("state"), q.Get("code"))
	switch {
	case errors.Is(err, auth.ErrOIDCDenied):
		redirectToLogin(w, r, "denied")
		return
	case err != nil:
		log.Printf("OIDC sign-in failed: %v", err)
		redirectToLogin(w, r, "failed")
		return
	}

	target := "/login"
	if ret.Redirect != "" {
		target += "?redirect=" + url.QueryEscape(ret.Redirect)
	}
	fragment := url.Values{"oidc_token": {token}, "check": {ret.Check}}
	http.Redirect(w, r, target+"#"+fragment.Encode(), http.StatusFound)
}

// redirectToLogin sends the browser to the login page with an error code the
// page turns into a message. A code rather than text, so a crafted link
// cannot put words on the page.
func redirectToLogin(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, "/login?oidc_error="+code, http.StatusFound)
}

// sameSitePath keeps a redirect target only if it is a path on this site.
// Browsers read a backslash as a slash, so "/\host" counts as "//host".
func sameSitePath(p string) string {
	if strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//") && !strings.Contains(p, "\\") {
		return p
	}
	return ""
}

// isHTTPS reports whether the browser reached LogDeck over HTTPS, directly or
// through a TLS-terminating proxy. A spoofed header only makes a cookie
// stricter.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
