package config

import "time"

// Config holds application configuration.
type Config struct {
	JWTSecret     string
	JWTExpiry     time.Duration
	ServerAddress string
}

// DefaultConfig returns a Config populated with sensible defaults.
// In production the JWT secret should come from an environment variable or
// secrets manager – never be hard-coded.
func DefaultConfig() *Config {
	return &Config{
		JWTSecret:     "",
		JWTExpiry:     24 * time.Hour,
		ServerAddress: ":9000",
	}
}
