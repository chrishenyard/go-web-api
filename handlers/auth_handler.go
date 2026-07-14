// Package handlers contains the HTTP handler structs for this application.
//
// AuthHandler (this file) is the only handler whose routes are public – no
// JWT is required for /api/auth/register or /api/auth/login.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/chrishenyard/go-web-api/config"
	"github.com/chrishenyard/go-web-api/models"
)

// AuthHandler handles user registration and login (public endpoints).
type AuthHandler struct {
	cfg   *config.Config
	store UserStore
}

// NewAuthHandler creates an AuthHandler backed by the supplied store and
// configuration.
func NewAuthHandler(cfg *config.Config, store UserStore) *AuthHandler {
	return &AuthHandler{cfg: cfg, store: store}
}

// Register handles POST /api/auth/register.
// It creates a new user account and returns a signed JWT.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request: invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "Bad Request: username, email and password are required", http.StatusBadRequest)
		return
	}

	if h.store.FindByUsername(req.Username) != nil {
		http.Error(w, "Conflict: username already taken", http.StatusConflict)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	user := &models.User{
		ID:           newID(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         models.RoleUser,
	}
	h.store.Save(user)

	token, err := h.issueToken(user)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// Login handles POST /api/auth/login.
// It validates credentials and returns a signed JWT on success.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request: invalid JSON", http.StatusBadRequest)
		return
	}

	user := h.store.FindByUsername(req.Username)
	if user == nil {
		http.Error(w, "Unauthorized: invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "Unauthorized: invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := h.issueToken(user)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// issueToken creates and signs a JWT for the supplied user.
func (h *AuthHandler) issueToken(u *models.User) (string, error) {
	now := time.Now()
	claims := &models.Claims{
		UserID:   u.ID,
		Username: u.Username,
		Role:     u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(h.cfg.JWTExpiry)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWTSecret))
}
