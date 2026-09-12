package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	gohashicorpvault "github.com/chrishenyard/go-hashicorp-vault"
	"github.com/joho/godotenv"
)

type Config struct {
	// OIDC configuration
	ClientID              string `env:"OIDC_CLIENT_ID" envDefault:""`
	ClientSecret          string `env:"OIDC_CLIENT_SECRET" envDefault:""`
	IssuerURL             string `env:"OIDC_ISSUER_URL" envDefault:""`
	Realm                 string `env:"OIDC_REALM" envDefault:""`
	LoginSuccessURL       string `env:"LOGIN_SUCCESS_URL" envDefault:"/dashboard"`
	PostLogoutRedirectURL string `env:"POST_LOGOUT_REDIRECT_URL" envDefault:"https://localhost:8081/"`

	// Web server configuration
	Port         string `env:"PORT" envDefault:"8081"`
	Host         string `env:"HOST" envDefault:"localhost"`
	RedirectHost string `env:"REDIRECT_HOST" envDefault:"localhost"`
	ServiceName  string `env:"SERVICE_NAME" envDefault:"go-web-api"`
	CertFilePath string `env:"CERT_FILE_PATH" envDefault:"$HOME/source/repos/go-web-api/certs/localhost.crt"`
	KeyFilePath  string `env:"KEY_FILE_PATH" envDefault:"$HOME/source/repos/go-web-api/certs/localhost.key"`
	// Logging configuration
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
	Env      string `env:"ENV" envDefault:"development"`
}

func NewConfig() (*Config, error) {
	cfg := &Config{
		ClientID:              os.Getenv("OIDC_CLIENT_ID"),
		IssuerURL:             os.Getenv("OIDC_ISSUER_URL"),
		Realm:                 os.Getenv("OIDC_REALM"),
		Port:                  os.Getenv("PORT"),
		Host:                  os.Getenv("HOST"),
		RedirectHost:          os.Getenv("REDIRECT_HOST"),
		ServiceName:           os.Getenv("SERVICE_NAME"),
		CertFilePath:          os.Getenv("CERT_FILE_PATH"),
		KeyFilePath:           os.Getenv("KEY_FILE_PATH"),
		LogLevel:              os.Getenv("LOG_LEVEL"),
		LoginSuccessURL:       os.Getenv("LOGIN_SUCCESS_URL"),
		PostLogoutRedirectURL: os.Getenv("POST_LOGOUT_REDIRECT_URL"),
	}

	secrets, err := GetVaultSecrets([]string{"OIDC_CLIENT_SECRET"})
	if err != nil {
		fmt.Printf("Error retrieving secrets from Vault: %v\n", err)
		return nil, err
	}
	cfg.ClientSecret = secrets["OIDC_CLIENT_SECRET"]

	return cfg, nil
}

func (cfg *Config) IsDevelopment() bool {
	return strings.ToLower(cfg.Env) == "development"
}

func getOptions() (options *gohashicorpvault.Options, err error) {
	envs, err := getEnv([]string{
		"VAULT_SECRET_ID",
	})
	if err != nil {
		return nil, fmt.Errorf("error reading .env file: %v", err)
	}

	options = &gohashicorpvault.Options{
		Address:                       os.Getenv("VAULT_ADDR"),
		AuthMethod:                    os.Getenv("VAULT_AUTH_METHOD"),
		KubernetesJwtPath:             os.Getenv("VAULT_KUBERNETES_JWT_PATH"),
		RoleId:                        os.Getenv("VAULT_ROLE_ID"),
		RoleName:                      os.Getenv("VAULT_ROLE_NAME"),
		SecretId:                      envs["VAULT_SECRET_ID"],
		MountPoint:                    os.Getenv("VAULT_MOUNT_POINT"),
		SecretPath:                    os.Getenv("VAULT_SECRET_PATH"),
		AllowInvalidServerCertificate: os.Getenv("VAULT_ALLOW_INVALID_SERVER_CERTIFICATE") == "true",
	}

	return options, nil
}

func GetVaultSecrets(keys []string) (map[string]string, error) {
	options, err := getOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to get Vault options: %v", err)
	}

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

func getEnv(keys []string) (map[string]string, error) {
	var envFilePath = os.Getenv("ENV_FILE_PATH")
	var envs map[string]string
	envs, err := godotenv.Read(envFilePath)
	if err != nil {
		return nil, fmt.Errorf("error reading .env file: %v", err)
	}

	result := make(map[string]string)
	for _, key := range keys {
		if value, ok := envs[key]; ok {
			result[key] = value
		} else {
			return nil, fmt.Errorf("%s is required but not found in .env file", key)
		}
	}
	return result, nil
}

func (cfg *Config) GetLogLevel() slog.Level {
	switch strings.ToUpper(cfg.LogLevel) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo // Safe fallback
	}
}
