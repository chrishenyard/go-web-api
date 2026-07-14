// UserHandler is an example of class-level authentication.
//
// The entire struct is registered behind [middleware.RequireAuth], meaning
// every method (List, Profile, Delete) automatically requires a valid JWT.
// Individual methods may add further checks (e.g. role checks) on top of the
// class-level gate – that is what Delete demonstrates: function-level
// authorization layered above class-level authentication.
package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/chrishenyard/go-web-api/middleware"
	"github.com/chrishenyard/go-web-api/models"
)

// UserHandler handles routes that require at minimum a valid JWT (class-level
// authentication).  All methods are protected by default because the entire
// struct is wrapped with [middleware.RequireAuth] when routes are registered.
type UserHandler struct {
	store UserStore
}

// NewUserHandler creates a UserHandler backed by the supplied store.
func NewUserHandler(store UserStore) *UserHandler {
	return &UserHandler{store: store}
}

// Profile handles GET /api/users/profile.
// Class-level: requires a valid JWT (enforced by RequireAuth wrapper).
func (h *UserHandler) Profile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	claims := middleware.ClaimsFromContext(r.Context())
	user := h.store.FindByID(claims.UserID)
	if user == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}

// List handles GET /api/users/.
// Class-level: requires a valid JWT (enforced by RequireAuth wrapper).
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	users := h.store.All()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(users)
}

// Delete handles DELETE /api/users/{id}.
// Class-level: requires a valid JWT (enforced by RequireAuth wrapper).
// Function-level: additionally requires the "admin" role, checked here
// via [middleware.RequireRole].  This is the function-level authorization
// layer – only administrators may delete accounts.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Function-level authorization: admin role required.
	middleware.RequireRole(models.RoleAdmin, h.deleteHandler)(w, r)
}

// deleteHandler is the actual deletion logic, called only after the
// function-level role check in Delete has passed.
func (h *UserHandler) deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract /{id} from the path prefix /api/users/.
	id := strings.TrimPrefix(r.URL.Path, "/api/users/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		http.Error(w, "Bad Request: missing user id", http.StatusBadRequest)
		return
	}

	if !h.store.Delete(id) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
