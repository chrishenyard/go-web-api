package config

import (
	"fmt"
	"os"

	gohashicorpvault "github.com/chrishenyard/go-hashicorp-vault"
)

type Config struct {
	// OIDC configuration
	ClientID     string `env:"OIDC_CLIENT_ID" envDefault:""`
	ClientSecret string `env:"OIDC_CLIENT_SECRET" envDefault:""`
	IssuerURL    string `env:"OIDC_ISSUER_URL" envDefault:""`
	Realm        string `env:"OIDC_REALM" envDefault:""`

	// Web server configuration
	Port         string `env:"PORT" envDefault:"8081"`
	Host         string `env:"HOST" envDefault:"localhost"`
	RedirectHost string `env:"REDIRECT_HOST" envDefault:"localhost"`
	// Logging configuration
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
}

func NewConfig() (*Config, error) {
	cfg := &Config{
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		IssuerURL:    os.Getenv("OIDC_ISSUER_URL"),
		Realm:        os.Getenv("OIDC_REALM"),
		Port:         os.Getenv("PORT"),
		Host:         os.Getenv("HOST"),
		RedirectHost: os.Getenv("REDIRECT_HOST"),
		LogLevel:     os.Getenv("LOG_LEVEL"),
	}

	secrets, err := GetVaultSecrets([]string{"OIDC_CLIENT_SECRET"})
	if err != nil {
		fmt.Printf("Error retrieving secrets from Vault: %v\n", err)
		return nil, err
	}
	cfg.ClientSecret = secrets["OIDC_CLIENT_SECRET"]

	return cfg, nil
}

func getOptions() (options *gohashicorpvault.Options) {
	options = &gohashicorpvault.Options{
		Address:                       os.Getenv("VAULT_ADDR"),
		AuthMethod:                    os.Getenv("VAULT_AUTH_METHOD"),
		KubernetesJwtPath:             os.Getenv("VAULT_KUBERNETES_JWT_PATH"),
		RoleId:                        os.Getenv("VAULT_ROLE_ID"),
		RoleName:                      os.Getenv("VAULT_ROLE_NAME"),
		SecretId:                      os.Getenv("VAULT_SECRET_ID"),
		MountPoint:                    os.Getenv("VAULT_MOUNT_POINT"),
		SecretPath:                    os.Getenv("VAULT_SECRET_PATH"),
		AllowInvalidServerCertificate: os.Getenv("VAULT_ALLOW_INVALID_SERVER_CERTIFICATE") == "true",
	}

	return options
}

func GetVaultSecrets(keys []string) (map[string]string, error) {
	options := getOptions()
	resp, err := gohashicorpvault.GetSecrets(options)
	if err != nil {
		return nil, fmt.Errorf("failed to get secrets from vault: %v", err)
	}

	secrets := make(map[string]string)

	for _, key := range keys {
		if value, ok := resp.Data.Data[key].(string); ok && value != "" {
			secrets[key] = value
		} else {
			return nil, fmt.Errorf("%s is required but not found in Vault", key)
		}
	}

	return secrets, nil
}
