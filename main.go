package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	auth "github.com/chrishenyard/go-oidc"
	"github.com/chrishenyard/go-web-api/config"
)

const (
	transactionCookieName = "oidc_transaction"
	sessionCookieName     = "oidc_session"
)

func main() {
	var cfg = config.NewConfig()
	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func run(cfg *config.Config) error {
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	startupCtx, cancel := context.WithTimeout(sigCtx, 10*time.Second)
	defer cancel()

	handler, err := newHttpHandler(startupCtx, cfg)
	if err != nil {
		return fmt.Errorf("initialize HTTP handler: %w", err)
	}

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Handler:           handler,
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
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-sigCtx.Done():
		stop()
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	return nil
}

func newHttpHandler(startupCtx context.Context, cfg *config.Config) (http.Handler, error) {
	store := auth.NewMemoryStore()

	clientSecret := os.Getenv("OIDC_CLIENT_SECRET")
	if clientSecret == "" {
		return nil, fmt.Errorf("OIDC_CLIENT_SECRET is required")
	}

	authClient, err := auth.New(
		startupCtx,
		auth.Config{
			IssuerURL: cfg.IssuerURL,

			ClientID:     cfg.ClientID,
			ClientSecret: clientSecret,
			RedirectURL:  fmt.Sprintf("http://%s:%s/callback", cfg.Host, cfg.Port),

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
		return nil, fmt.Errorf("create OIDC client: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/login", authClient.LoginHandler())
	mux.Handle("/callback", authClient.CallbackHandler())
	mux.Handle("/logout", authClient.LogoutHandler())
	mux.Handle("/dashboard", authClient.RequireRole("user", http.HandlerFunc(handleDashboard)))
	mux.Handle("/admin", authClient.RequireRole("admin", http.HandlerFunc(handleAdmin)))

	return mux, nil
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "authenticated claims are unavailable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	displayName := claims.Email
	if displayName == "" {
		displayName = "authenticated user"
	}

	_, _ = fmt.Fprintf(w, "Welcome to the user dashboard, %s!", displayName)
}

func handleAdmin(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("Welcome to the restricted admin control panel!"))
}
