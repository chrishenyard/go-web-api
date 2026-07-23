package tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	keycloak "github.com/stillya/testcontainers-keycloak"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var keycloakContainer *keycloak.KeycloakContainer

func Test_UserCanAuthenticate(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	authServerURL, err := keycloakContainer.GetAuthServerURL(ctx)
	if err != nil {
		t.Fatalf("GetAuthServerURL() error: %v", err)
	}

	password := os.Getenv("KEYCLOAK_USER_PASSWORD")

	tokens, err := authenticateUser(
		ctx,
		authServerURL,
		"Golang",
		"go-web-api-client",
		"user@example.com",
		password,
	)
	if err != nil {
		t.Fatalf("authenticateUser() error: %v", err)
	}

	if tokens.AccessToken == "" {
		t.Fatal("expected an access token, but received an empty value")
	}

	if tokens.TokenType != "Bearer" {
		t.Errorf(
			"expected token type Bearer, got %q",
			tokens.TokenType,
		)
	}

	if tokens.ExpiresIn <= 0 {
		t.Errorf(
			"expected a positive token expiration, got %d",
			tokens.ExpiresIn,
		)
	}

	t.Logf(
		"user authenticated successfully; token expires in %d seconds",
		tokens.ExpiresIn,
	)
}

// func Test_Example(t *testing.T) {
// 	ctx := context.Background()

// 	authServerURL, err := keycloakContainer.GetAuthServerURL(ctx)
// 	if err != nil {
// 		t.Errorf("GetAuthServerURL() error = %v", err)
// 		return
// 	}

// 	fmt.Println(authServerURL)
// 	// Output:
// 	// http://localhost:32768/auth
// }

// func Test_Admin_Client(t *testing.T) {
// 	ctx := context.Background()

// 	adminClient, err := keycloakContainer.GetAdminClient(ctx)
// 	if err != nil {
// 		t.Errorf("GetAdminClient() error = %v", err)
// 		return
// 	}

// 	fmt.Println(adminClient)
// }

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
		keycloak.WithRealmImportFile("../testdata/realm-export.json"),
		keycloak.WithAdminUsername("admin"),
		keycloak.WithAdminPassword("admin"),
	)
}
