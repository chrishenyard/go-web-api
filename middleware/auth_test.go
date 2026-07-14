package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/chrishenyard/go-web-api/middleware"
	"github.com/chrishenyard/go-web-api/models"
)

const testSecret = "test-secret-key"

// makeToken creates a signed JWT for the given user ID, username, and role.
func makeToken(t *testing.T, userID, username string, role models.Role, expiry time.Duration) string {
	t.Helper()
	now := time.Now()
	claims := &models.Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("makeToken: %v", err)
	}
	return tok
}

// okHandler is a trivial handler that returns 200 so we can verify middleware
// lets the request through.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// TestRequireAuth_MissingHeader verifies that requests without an Authorization
// header are rejected with 401.
func TestRequireAuth_MissingHeader(t *testing.T) {
	h := middleware.RequireAuth(testSecret, okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// TestRequireAuth_InvalidToken verifies that a tampered token is rejected.
func TestRequireAuth_InvalidToken(t *testing.T) {
	h := middleware.RequireAuth(testSecret, okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "******")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// TestRequireAuth_ExpiredToken verifies that expired tokens are rejected.
func TestRequireAuth_ExpiredToken(t *testing.T) {
	tok := makeToken(t, "u1", "alice", models.RoleUser, -time.Hour)
	h := middleware.RequireAuth(testSecret, okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// TestRequireAuth_ValidToken verifies that a valid token passes the middleware
// and the claims are stored in the request context.
func TestRequireAuth_ValidToken(t *testing.T) {
	tok := makeToken(t, "u1", "alice", models.RoleUser, time.Hour)

	var gotClaims *models.Claims
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClaims = middleware.ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	h := middleware.RequireAuth(testSecret, inner)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if gotClaims == nil {
		t.Fatal("claims not stored in context")
	}
	if gotClaims.UserID != "u1" {
		t.Errorf("unexpected UserID: %s", gotClaims.UserID)
	}
}

// TestRequireRole_WrongRole ensures a user without the required role gets 403.
func TestRequireRole_WrongRole(t *testing.T) {
	// Build a context that already has user claims (as RequireAuth would set).
	tok := makeToken(t, "u2", "bob", models.RoleUser, time.Hour)

	inner := middleware.RequireRole(models.RoleAdmin, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with RequireAuth so claims end up in the context.
	h := middleware.RequireAuth(testSecret, inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestRequireRole_CorrectRole verifies that a user with the required role is
// allowed through.
func TestRequireRole_CorrectRole(t *testing.T) {
	tok := makeToken(t, "u3", "carol", models.RoleAdmin, time.Hour)

	inner := middleware.RequireRole(models.RoleAdmin, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := middleware.RequireAuth(testSecret, inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
