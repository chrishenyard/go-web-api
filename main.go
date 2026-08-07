package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	auth "github.com/chrishenyard/go-oidc"
)

const (
	transactionCookieName = "oidc_transaction"
	sessionCookieName     = "oidc_session"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	startupCtx, cancel := context.WithTimeout(sigCtx, 10*time.Second)
	defer cancel()

	server := &http.Server{
		Addr:              ":8081",
		Handler:           newHttpHandler(startupCtx),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	srvErr := make(chan error, 1)
	go func() {
		log.Println("Running HTTP server...")
		srvErr <- server.ListenAndServe()
	}()

	select {
	case err := <-srvErr:
		return err
	case <-sigCtx.Done():
		stop()
	}

	// Shutdown the server gracefully.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	return nil
}

func newHttpHandler(startupCtx context.Context) http.Handler {
	store := auth.NewMemoryStore()

	authClient, err := auth.New(
		startupCtx,
		auth.Config{
			IssuerURL: "http://localhost:8080/realms/Golang_Private",

			ClientID:     "go-web-api-client-private",
			ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
			RedirectURL:  "http://localhost:8081/callback",

			RequestedScopes: []string{
				"profile",
				"email",
				"roles",
			},

			Authorization: auth.AuthorizationConfig{
				RoleClaimPaths: []string{
					"roles",
					"groups",
					"realm_access.roles",
					"resource_access.{client_id}.roles",
				},
				ScopeClaimPaths: []string{
					"scope",
					"scp",
				},
			},

			Store: store,

			TransactionCookieName: transactionCookieName,
			SessionCookieName:     sessionCookieName,

			TransactionLifetime: 5 * time.Minute,
			SessionLifetime:     8 * time.Hour,

			CookieSecure:   false,
			CookieSameSite: http.SameSiteLaxMode,

			LoginSuccessURL: "/dashboard",
		},
	)

	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/login", authClient.LoginHandler())
	mux.Handle("/callback", authClient.CallbackHandler())
	mux.Handle("/logout", authClient.LogoutHandler())
	mux.Handle("/dashboard", authClient.RequireRole("user", http.HandlerFunc(handleDashboard)))
	mux.Handle("/admin", authClient.RequireRole("admin", http.HandlerFunc(handleAdmin)))

	return mux
}

func handleDashboard(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "authenticated claims are unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "Welcome to the user dashboard, %s!", claims.Email)
}

func handleAdmin(
	w http.ResponseWriter,
	_ *http.Request,
) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("Welcome to the restricted admin control panel!"))
}
