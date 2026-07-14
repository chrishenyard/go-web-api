// Package middleware provides HTTP middleware used across the application.
//
// Class-level authentication is implemented here: wrapping an entire handler
// group (struct) with [RequireAuth] enforces JWT verification for every route
// that belongs to that group.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/chrishenyard/go-web-api/models"
)

// claimsKey is an unexported type used as the context key for JWT claims,
// preventing collisions with keys set by other packages.
type claimsKey struct{}

// RequireAuth is a middleware that validates a Bearer JWT on every request.
// Applying it to a whole handler struct (or router group) implements
// class-level authentication – all methods of that struct are protected.
func RequireAuth(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr, err := extractBearerToken(r)
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}

		claims := &models.Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromContext retrieves JWT claims stored in the request context by
// [RequireAuth]. Returns nil if no claims are present.
func ClaimsFromContext(ctx context.Context) *models.Claims {
	c, _ := ctx.Value(claimsKey{}).(*models.Claims)
	return c
}

// RequireRole returns an HTTP handler that checks whether the authenticated
// user holds the expected role and calls next if so.  This implements
// function-level authorization: individual handler functions call
// RequireRole to enforce finer-grained access control on top of the
// class-level JWT check already performed by [RequireAuth].
func RequireRole(role models.Role, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromContext(r.Context())
		if claims == nil {
			http.Error(w, "Forbidden: missing claims", http.StatusForbidden)
			return
		}
		if claims.Role != role {
			http.Error(w, "Forbidden: insufficient permissions", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// extractBearerToken parses the Authorization header and returns the raw
// JWT string.
func extractBearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", errors.New("missing Authorization header")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errors.New("Authorization header must be 'Bearer <token>'")
	}
	return parts[1], nil
}
