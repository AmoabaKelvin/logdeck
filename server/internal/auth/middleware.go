package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

type contextKey string

// UserContextKey holds the request's authenticated identity, a models.User.
// Read it with UserFromContext.
const UserContextKey contextKey = "user"

// UserFromContext returns who made a request: a user's name and role, or
// "token:<name>" and the scope for an API token. ok is false when auth is
// disabled and nobody signed in.
func UserFromContext(ctx context.Context) (models.User, bool) {
	user, ok := ctx.Value(UserContextKey).(models.User)
	return user, ok
}

// APITokenLookup resolves a presented API token to its name and scope.
// Implementations must compare against stored hashes in constant time.
type APITokenLookup func(token string) (name, scope string, ok bool)

// DynamicMiddleware creates an auth middleware that resolves the auth service per request.
// If getService returns nil, auth is disabled and the request passes through.
// If lookupAPIToken is non-nil, bearer tokens with the API token prefix are
// authenticated against stored API tokens instead of as JWTs.
func DynamicMiddleware(getService func() *Service, lookupAPIToken APITokenLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			svc := getService()
			if svc == nil {
				next.ServeHTTP(w, r)
				return
			}
			validateAndServe(svc, lookupAPIToken, next, w, r)
		})
	}
}

// validateAndServe extracts and validates the bearer token (API token or JWT),
// then serves the request.
func validateAndServe(svc *Service, lookupAPIToken APITokenLookup, next http.Handler, w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	var tokenString string

	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}
		tokenString = parts[1]
	} else {
		tokenString = r.URL.Query().Get("token")
		if tokenString == "" {
			http.Error(w, "Authorization header or token query parameter required", http.StatusUnauthorized)
			return
		}
	}

	// API tokens are identified by their fixed prefix; a JWT never starts
	// with it, so there is no fallthrough between the two schemes.
	if strings.HasPrefix(tokenString, APITokenPrefix) {
		if lookupAPIToken != nil {
			if name, scope, ok := lookupAPIToken(tokenString); ok {
				serveAs(models.User{Username: "token:" + name, Role: NormalizeAPITokenScope(scope)}, next, w, r)
				return
			}
		}
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	claims, err := svc.VerifyToken(tokenString)
	if err != nil {
		if errors.Is(err, ErrTokenExpired) {
			http.Error(w, "Token has expired", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	user, ok := svc.SessionUser(claims)
	if !ok {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	serveAs(user, next, w, r)
}

// serveAs serves the request as user, refusing a mutating request from a
// read-only user or token.
func serveAs(user models.User, next http.Handler, w http.ResponseWriter, r *http.Request) {
	if user.Role == RoleRead && isMutatingRequest(r) {
		http.Error(w, readOnlyMessage(user), http.StatusForbidden)
		return
	}
	ctx := context.WithValue(r.Context(), UserContextKey, user)
	next.ServeHTTP(w, r.WithContext(ctx))
}

func readOnlyMessage(user models.User) string {
	if strings.HasPrefix(user.Username, "token:") {
		return "This API token is read-only and cannot perform this operation"
	}
	return "This account is read-only and cannot perform this operation"
}

// isMutatingRequest reports whether a request mutates state: anything other
// than GET/HEAD/OPTIONS. GET routes that are nonetheless off-limits to read
// tokens (exec, container env, settings) attach DenyReadScope explicitly.
func isMutatingRequest(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// DenyReadScope is a route middleware that rejects requests from a read-only
// user or a read-scoped API token. Attach it to routes that are read-shaped
// but sensitive or mutating (exec, container env, settings).
func DenyReadScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, ok := UserFromContext(r.Context()); ok && user.Role == RoleRead {
			http.Error(w, readOnlyMessage(user), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// IsReadScoped reports whether the request was authenticated by a read-only
// user or a read-scoped API token.
func IsReadScoped(r *http.Request) bool {
	user, ok := UserFromContext(r.Context())
	return ok && user.Role == RoleRead
}
