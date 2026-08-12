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
	Port string `env:"PORT" envDefault:"8081"`
	Host string `env:"HOST" envDefault:"localhost"`
	// Logging configuration
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
}

func NewConfig() (*Config, error) {
	cfg := &Config{
		ClientID:  os.Getenv("OIDC_CLIENT_ID"),
		IssuerURL: os.Getenv("OIDC_ISSUER_URL"),
		Realm:     os.Getenv("OIDC_REALM"),
		Port:      os.Getenv("PORT"),
		Host:      os.Getenv("HOST"),
		LogLevel:  os.Getenv("LOG_LEVEL"),
	}

	err := setSecretsFromVault(cfg)
	if err != nil {
		fmt.Printf("Error retrieving secrets from Vault: %v\n", err)
		return nil, err
	}

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

func setSecretsFromVault(cfg *Config) error {
	options := getOptions()
	resp, err := gohashicorpvault.GetSecrets(options)
	if err != nil {
		return fmt.Errorf("failed to get secrets from vault: %v", err)
	}

	if cfg.ClientSecret = resp.Data.Data["OIDC_CLIENT_SECRET"].(string); cfg.ClientSecret == "" {
		return fmt.Errorf("OIDC_CLIENT_SECRET is required but not found in Vault")
	}

	return nil
}
