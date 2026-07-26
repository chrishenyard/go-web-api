package tests

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	keycloak "github.com/stillya/testcontainers-keycloak"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var keycloakContainer *keycloak.KeycloakContainer

func Test_UserCanAuthenticateWithPKCE(t *testing.T) {
	const (
		realm    = "Golang"
		clientID = "go-web-api-client"
		username = "user@example.com"
	)

	password := os.Getenv("KEYCLOAK_USER_PASSWORD")
	if password == "" {
		t.Fatal("KEYCLOAK_USER_PASSWORD is not set")
	}

	testCtx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	authServerURL, err := keycloakContainer.GetAuthServerURL(testCtx)
	if err != nil {
		t.Fatalf("get Keycloak URL: %v", err)
	}

	callback, err := startCallbackServer()
	if err != nil {
		t.Fatalf("start callback server: %v", err)
	}
	defer func() {
		if err := callback.Close(); err != nil {
			t.Logf("close callback server: %v", err)
		}
	}()

	codeVerifier, codeChallenge, err := createPKCE()
	if err != nil {
		t.Fatalf("create PKCE values: %v", err)
	}

	state, err := createRandomState()
	if err != nil {
		t.Fatalf("create state: %v", err)
	}

	authorizationURL := buildAuthorizationURL(
		authServerURL,
		realm,
		clientID,
		callback.RedirectURL,
		state,
		codeChallenge,
	)

	browserCtx, browserCancel := chromedp.NewContext(testCtx)
	defer browserCancel()

	err = chromedp.Run(
		browserCtx,
		chromedp.Navigate(authorizationURL),
		chromedp.WaitVisible(`#username`, chromedp.ByQuery),
		chromedp.SendKeys(`#username`, username, chromedp.ByQuery),
		chromedp.SendKeys(`#password`, password, chromedp.ByQuery),
		chromedp.Click(`#kc-login`, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("complete Keycloak login: %v", err)
	}

	var callbackResult callbackResult

	select {
	case callbackResult = <-callback.Results:
	case <-time.After(10 * time.Second):
		var currentURL string
		var loginError string

		_ = chromedp.Run(
			browserCtx,
			chromedp.Location(&currentURL),
			chromedp.Text(
				`.kc-feedback-text`,
				&loginError,
				chromedp.ByQuery,
				chromedp.NodeVisible,
			),
		)

		t.Fatalf(
			"timed out waiting for callback; browser URL=%q; Keycloak error=%q",
			currentURL,
			loginError,
		)
	case <-testCtx.Done():
		t.Fatal("timed out waiting for Keycloak callback")
	}

	if callbackResult.Error != "" {
		t.Fatalf(
			"Keycloak callback returned error: %s",
			callbackResult.Error,
		)
	}

	if callbackResult.Code == "" {
		t.Fatal("callback did not contain an authorization code")
	}

	if callbackResult.State != state {
		t.Fatalf(
			"state mismatch: expected %q, received %q",
			state,
			callbackResult.State,
		)
	}

	tokens, err := exchangeAuthorizationCode(
		testCtx,
		authServerURL,
		realm,
		clientID,
		callback.RedirectURL,
		callbackResult.Code,
		codeVerifier,
	)
	if err != nil {
		t.Fatalf("exchange authorization code: %v", err)
	}

	if tokens.AccessToken == "" {
		t.Fatal("expected a non-empty access token")
	}

	if tokens.IDToken == "" {
		t.Fatal("expected a non-empty ID token")
	}

	if tokens.TokenType != "Bearer" {
		t.Errorf(
			"expected Bearer token, received %q",
			tokens.TokenType,
		)
	}

	t.Logf(
		"PKCE authentication succeeded; token expires in %d seconds",
		tokens.ExpiresIn,
	)
}

func createRandomState() (string, error) {
	value := make([]byte, 32)

	if _, err := rand.Read(value); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(value), nil
}

func TestMain(m *testing.M) {
	defer func() {
		if r := recover(); r != nil {
			shutDown()
			fmt.Println("Panic")
		}
	}()
	setup()
	code := m.Run()
	shutDown()
	os.Exit(code)
}

func setup() {
	var err error
	ctx := context.Background()
	keycloakContainer, err = RunContainer(ctx)
	if err != nil {
		panic(err)
	}
}

func shutDown() {
	ctx := context.Background()
	err := keycloakContainer.Terminate(ctx)
	if err != nil {
		panic(err)
	}
}

func RunContainer(ctx context.Context) (*keycloak.KeycloakContainer, error) {
	return keycloak.Run(ctx,
		"keycloak/keycloak",
		testcontainers.WithWaitStrategy(wait.ForListeningPort("8080/tcp").WithStartupTimeout(60*time.Second)),
		keycloak.WithContextPath(""),
		keycloak.WithRealmImportFile("../testdata/realm_export.json"),
		keycloak.WithAdminUsername("admin"),
		keycloak.WithAdminPassword("admin"),
	)
}
