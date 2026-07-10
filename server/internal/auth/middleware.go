package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

type contextKey string

const UserContextKey contextKey = "user"

// APITokenLookup resolves a presented API token to its name. Implementations
// must compare against stored hashes in constant time.
type APITokenLookup func(token string) (name string, ok bool)

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

// Middleware creates a static authentication middleware for a fixed auth service.
func Middleware(authService *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			validateAndServe(authService, nil, next, w, r)
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
			if name, ok := lookupAPIToken(tokenString); ok {
				user := models.User{Username: "token:" + name, Role: "admin"}
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))
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

	user := GetUserFromClaims(claims)
	ctx := context.WithValue(r.Context(), UserContextKey, user)
	next.ServeHTTP(w, r.WithContext(ctx))
}
