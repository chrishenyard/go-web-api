package config

import "os"

type Config struct {
	// OIDC configuration
	ClientID     string `env:"OIDC_CLIENT_ID" envDefault:""`
	ClientSecret string `env:"OIDC_CLIENT_SECRET" envDefault:""`
	IssuerURL    string `env:"OIDC_ISSUER_URL" envDefault:""`
	Realm		 string `env:"OIDC_REALM" envDefault:""`

	// Web server configuration
	Port string `env:"PORT" envDefault:"8081"`
	Host string `env:"HOST" envDefault:"localhost"`
	// Logging configuration
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
}

func NewConfig() *Config {
	return &Config{
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		IssuerURL:    os.Getenv("OIDC_ISSUER_URL"),
		Realm:        os.Getenv("OIDC_REALM"),
		Port:         os.Getenv("PORT"),
		Host:         os.Getenv("HOST"),
		LogLevel:     os.Getenv("LOG_LEVEL"),
	}
}
