package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	auth "github.com/chrishenyard/go-oidc"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	store := auth.NewMemoryStore()

	authClient, err := auth.New(
		ctx,
		auth.Config{
			IssuerURL: "http://127.0.0.1:8080/realms/Golang_Private",

			ClientID:     "go-web-api-client-private",
			ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),

			RedirectURL: "http://127.0.0.1:8081/callback",

			Scopes: []string{
				"openid",
				"profile",
				"email",
				"roles",

				// Include this only when you specifically want
				// Keycloak offline sessions.
				// "offline_access",
			},

			Store: store,

			CookieSecure: false,

			TransactionLifetime: 5 * time.Minute,
			SessionLifetime:     8 * time.Hour,

			LoginSuccessURL: "/dashboard",

			ErrorHandler: authenticationErrorHandler,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"initialize authentication: %w",
			err,
		)
	}

	mux := http.NewServeMux()

	mux.Handle("/login", authClient.LoginHandler())
	mux.Handle("/callback", authClient.CallbackHandler())
	mux.Handle("/logout", authClient.LogoutHandler())

	mux.Handle(
		"/dashboard",
		authClient.RequireRole(
			"user",
			http.HandlerFunc(handleDashboard),
		),
	)

	mux.Handle(
		"/admin",
		authClient.RequireRole(
			"admin",
			http.HandlerFunc(handleAdmin),
		),
	)

	server := &http.Server{
		Addr:              ":8081",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("server listening on %s", server.Addr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	return nil
}

func authenticationErrorHandler(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	/*
		Log the complete internal error on the server.
		The default handler sends only a safe general message
		to the client.
	*/
	log.Printf(
		"authentication error: method=%s path=%s error=%v",
		r.Method,
		r.URL.Path,
		err,
	)

	auth.DefaultErrorHandler(w, r, err)
}

func handleDashboard(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(
			w,
			"authenticated claims are unavailable",
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set(
		"Content-Type",
		"text/plain; charset=utf-8",
	)

	_, _ = fmt.Fprintf(
		w,
		"Welcome to the user dashboard, %s!",
		claims.Email,
	)
}

func handleAdmin(
	w http.ResponseWriter,
	_ *http.Request,
) {
	w.Header().Set(
		"Content-Type",
		"text/plain; charset=utf-8",
	)

	_, _ = w.Write(
		[]byte(
			"Welcome to the restricted admin control panel!",
		),
	)
}
