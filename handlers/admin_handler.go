// AdminHandler is a second example of class-level authentication combined with
// function-level authorization.
//
// Class-level: every route under /api/admin/ is wrapped with
// [middleware.RequireAuth] (JWT must be valid).
//
// Function-level: each individual handler method also calls
// [middleware.RequireRole] with models.RoleAdmin, so only users with the
// "admin" role can reach the actual logic.  This two-tier design means:
//   - Any authenticated user that hits /api/admin/* gets a 403 if they are
//     not an admin (function-level gate).
//   - Unauthenticated requests never even reach AdminHandler code (class-level
//     gate in RequireAuth).
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/chrishenyard/go-web-api/middleware"
	"github.com/chrishenyard/go-web-api/models"
)

// AdminHandler handles privileged routes that require both a valid JWT and
// the "admin" role.
type AdminHandler struct {
	store UserStore
}

// NewAdminHandler creates an AdminHandler backed by the supplied store.
func NewAdminHandler(store UserStore) *AdminHandler {
	return &AdminHandler{store: store}
}

// Stats handles GET /api/admin/stats.
// Class-level: JWT required (handled by RequireAuth wrapper in routing).
// Function-level: "admin" role required (checked here via RequireRole).
func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	// Function-level authorization: only admins may view stats.
	middleware.RequireRole(models.RoleAdmin, h.statsHandler)(w, r)
}

func (h *AdminHandler) statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	users := h.store.All()
	stats := map[string]interface{}{
		"total_users": len(users),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

// PromoteUser handles POST /api/admin/promote.
// Class-level: JWT required.
// Function-level: "admin" role required.
func (h *AdminHandler) PromoteUser(w http.ResponseWriter, r *http.Request) {
	middleware.RequireRole(models.RoleAdmin, h.promoteUserHandler)(w, r)
}

func (h *AdminHandler) promoteUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		http.Error(w, "Bad Request: user_id required", http.StatusBadRequest)
		return
	}

	user := h.store.FindByID(req.UserID)
	if user == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	user.Role = models.RoleAdmin
	h.store.Save(user)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
}
