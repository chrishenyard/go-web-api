package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	auth "github.com/chrishenyard/go-oidc"
	"github.com/chrishenyard/go-web-api/config"
	loggerMiddleware "github.com/chrishenyard/go-web-api/middleware"
)

const (
	transactionCookieName = "oidc_transaction"
	sessionCookieName     = "oidc_session"
)

func NewHttpHandler(startupCtx context.Context, cfg *config.Config) (http.Handler, error) {
	store := auth.NewMemoryStore()
	level := loggerMiddleware.ParseLogLevel(cfg.LogLevel)
	opts := &slog.HandlerOptions{
		Level: level,
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)

	authConfig := auth.Config{
		IssuerURL: cfg.IssuerURL,

		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  fmt.Sprintf("http://%s:%s/callback", cfg.RedirectHost, cfg.Port),

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

		LoggerFromContext: loggerMiddleware.LoggerFromContext,
	}

	authClient, err := auth.New(
		startupCtx,
		authConfig,
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
	mux.Handle("/", http.HandlerFunc(handleDefault))

	wrappedMux := loggerMiddleware.LoggingMiddleware(logger, mux)

	return wrappedMux, nil
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

func handleDefault(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("Welcome to the public area of the application!"))
}
