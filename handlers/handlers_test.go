package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/chrishenyard/go-web-api/config"
	"github.com/chrishenyard/go-web-api/handlers"
	"github.com/chrishenyard/go-web-api/middleware"
	"github.com/chrishenyard/go-web-api/models"
)

const authTestSecret = "handler-test-secret"

func newTestConfig() *config.Config {
	return &config.Config{
		JWTSecret: authTestSecret,
		JWTExpiry: time.Hour,
	}
}

func makeTestToken(t *testing.T, userID, username string, role models.Role) string {
	t.Helper()
	now := time.Now()
	claims := &models.Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(authTestSecret))
	if err != nil {
		t.Fatalf("makeTestToken: %v", err)
	}
	return tok
}

// ----- AuthHandler tests -----

func TestAuthHandler_Register_Success(t *testing.T) {
	store := handlers.NewMemoryUserStore()
	h := handlers.NewAuthHandler(newTestConfig(), store)

	body := `{"username":"alice","email":"alice@example.com","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["token"] == "" {
		t.Error("expected a token in the response")
	}
}

func TestAuthHandler_Register_DuplicateUsername(t *testing.T) {
	store := handlers.NewMemoryUserStore()
	h := handlers.NewAuthHandler(newTestConfig(), store)

	body := `{"username":"alice","email":"a@a.com","password":"pass"}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.Register(rr, req)
		if i == 1 && rr.Code != http.StatusConflict {
			t.Errorf("expected 409 on duplicate, got %d", rr.Code)
		}
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	store := handlers.NewMemoryUserStore()
	cfg := newTestConfig()
	h := handlers.NewAuthHandler(cfg, store)

	// Register first.
	regBody := `{"username":"bob","email":"bob@example.com","password":"hunter2"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(regBody))
	req.Header.Set("Content-Type", "application/json")
	h.Register(httptest.NewRecorder(), req)

	// Now login.
	loginBody := `{"username":"bob","password":"hunter2"}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Login(rr, req2)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["token"] == "" {
		t.Error("expected a token in login response")
	}
}

func TestAuthHandler_Login_WrongPassword(t *testing.T) {
	store := handlers.NewMemoryUserStore()
	h := handlers.NewAuthHandler(newTestConfig(), store)

	regBody := `{"username":"carol","email":"c@c.com","password":"correct"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(regBody))
	req.Header.Set("Content-Type", "application/json")
	h.Register(httptest.NewRecorder(), req)

	loginBody := `{"username":"carol","password":"wrong"}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Login(rr, req2)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// ----- UserHandler (class-level auth) tests -----

// withAuth wraps a handler with RequireAuth using the test secret.
func withAuth(h http.Handler) http.Handler {
	return middleware.RequireAuth(authTestSecret, h)
}

func TestUserHandler_Profile_NoToken(t *testing.T) {
	store := handlers.NewMemoryUserStore()
	h := withAuth(http.HandlerFunc(handlers.NewUserHandler(store).Profile))

	req := httptest.NewRequest(http.MethodGet, "/api/users/profile", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestUserHandler_Profile_WithValidToken(t *testing.T) {
	store := handlers.NewMemoryUserStore()
	// Pre-populate store with a user.
	user := &models.User{
		ID:       "uid1",
		Username: "dave",
		Email:    "dave@example.com",
		Role:     models.RoleUser,
	}
	store.Save(user)

	tok := makeTestToken(t, "uid1", "dave", models.RoleUser)
	h := withAuth(http.HandlerFunc(handlers.NewUserHandler(store).Profile))

	req := httptest.NewRequest(http.MethodGet, "/api/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestUserHandler_Delete_UserRole verifies function-level authorization:
// a regular user cannot delete accounts even though they hold a valid JWT.
func TestUserHandler_Delete_UserRole(t *testing.T) {
	store := handlers.NewMemoryUserStore()
	target := &models.User{ID: "tid1", Username: "target", Role: models.RoleUser}
	store.Save(target)

	tok := makeTestToken(t, "uid2", "attacker", models.RoleUser)
	h := withAuth(http.HandlerFunc(handlers.NewUserHandler(store).Delete))

	req := httptest.NewRequest(http.MethodDelete, "/api/users/tid1", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 (non-admin delete), got %d", rr.Code)
	}
}

// TestUserHandler_Delete_AdminRole verifies that an admin can delete users
// (function-level authorization grants access).
func TestUserHandler_Delete_AdminRole(t *testing.T) {
	store := handlers.NewMemoryUserStore()
	target := &models.User{ID: "tid2", Username: "victim", Role: models.RoleUser}
	store.Save(target)

	tok := makeTestToken(t, "admin1", "superuser", models.RoleAdmin)
	uh := handlers.NewUserHandler(store)

	// Simulate the route as registered in main.go: class-level auth wraps the
	// mux, and Delete itself applies function-level role check.
	inner := http.NewServeMux()
	inner.HandleFunc("/api/users/", uh.Delete)
	h := withAuth(inner)

	req := httptest.NewRequest(http.MethodDelete, "/api/users/tid2", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ----- AdminHandler (class + function-level auth) tests -----

func TestAdminHandler_Stats_RegularUser(t *testing.T) {
	store := handlers.NewMemoryUserStore()
	ah := handlers.NewAdminHandler(store)

	tok := makeTestToken(t, "u5", "eve", models.RoleUser)
	h := withAuth(http.HandlerFunc(ah.Stats))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestAdminHandler_Stats_AdminUser(t *testing.T) {
	store := handlers.NewMemoryUserStore()
	ah := handlers.NewAdminHandler(store)

	tok := makeTestToken(t, "u6", "frank", models.RoleAdmin)
	h := withAuth(http.HandlerFunc(ah.Stats))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminHandler_Stats_NoToken(t *testing.T) {
	store := handlers.NewMemoryUserStore()
	h := withAuth(http.HandlerFunc(handlers.NewAdminHandler(store).Stats))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
