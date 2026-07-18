package main

import (
	"log"
	"net/http"
	"os"

	gohashicorpvault "github.com/chrishenyard/go-hashicorp-vault"
	"github.com/chrishenyard/go-web-api/config"
	"github.com/chrishenyard/go-web-api/handlers"
	"github.com/chrishenyard/go-web-api/middleware"
)

func main() {
	cfg := config.DefaultConfig()
	setSecretsFromVault(cfg)

	store := handlers.NewMemoryUserStore()

	authHandler := handlers.NewAuthHandler(cfg, store)
	userHandler := handlers.NewUserHandler(store)
	adminHandler := handlers.NewAdminHandler(store)

	mux := http.NewServeMux()

	// ── Public routes (no JWT required) ───────────────────────────────────
	mux.HandleFunc("/api/auth/register", authHandler.Register)
	mux.HandleFunc("/api/auth/login", authHandler.Login)

	// ── Class-level authentication: UserHandler ───────────────────────────
	// Every route under /api/users/ is wrapped with RequireAuth.
	// All methods of UserHandler are therefore protected by a single
	// class-level JWT gate.  The Delete method adds a function-level role
	// check on top of this.
	userRoutes := http.NewServeMux()
	userRoutes.HandleFunc("/api/users/profile", userHandler.Profile)
	userRoutes.HandleFunc("/api/users/", userHandler.Delete) // handles DELETE /api/users/{id}
	userRoutes.HandleFunc("/api/users", userHandler.List)
	mux.Handle("/api/users", middleware.RequireAuth(cfg.JWTSecret, userRoutes))
	mux.Handle("/api/users/", middleware.RequireAuth(cfg.JWTSecret, userRoutes))

	// ── Class-level authentication: AdminHandler ──────────────────────────
	// Every route under /api/admin/ is wrapped with RequireAuth.
	// Each AdminHandler method additionally enforces the "admin" role
	// (function-level authorization).
	adminRoutes := http.NewServeMux()
	adminRoutes.HandleFunc("/api/admin/stats", adminHandler.Stats)
	adminRoutes.HandleFunc("/api/admin/promote", adminHandler.PromoteUser)
	mux.Handle("/api/admin/", middleware.RequireAuth(cfg.JWTSecret, adminRoutes))

	log.Printf("server listening on %s", cfg.ServerAddress)
	if err := http.ListenAndServe(cfg.ServerAddress, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getOptions() (options *gohashicorpvault.Options) {
	options = &gohashicorpvault.Options{
		Address:                       os.Getenv("VAULT_ADDR"),
		AuthMethod:                    "approle",
		KubernetesJwtPath:             os.Getenv("VAULT_KUBERNETES_JWT_PATH"),
		RoleId:                        os.Getenv("VAULT_ROLE_ID"),
		RoleName:                      os.Getenv("VAULT_ROLE_NAME"),
		SecretId:                      os.Getenv("VAULT_SECRET_ID"),
		MountPoint:                    os.Getenv("VAULT_MOUNT_POINT"),
		SecretPath:                    os.Getenv("VAULT_SECRET_PATH"),
		AllowInvalidServerCertificate: true,
	}
	return options
}

func setSecretsFromVault(cfg *config.Config) {
	options := getOptions()
	resp, err := gohashicorpvault.GetSecrets(options)
	if err != nil {
		log.Fatalf("failed to get secrets from vault: %v", err)
	}

	if cfg.JWTSecret = resp.Data.Data["jwt_secret"].(string); cfg.JWTSecret == "" {
		log.Fatalf("JWT secret is empty")
	}
}
