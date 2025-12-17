package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration.
type Config struct {
	Database DatabaseConfig
	GRPC     GRPCConfig
	Auth     AuthConfig
}

// DatabaseConfig contains database-related settings.
type DatabaseConfig struct {
	Path string // SQLite database file path
}

// GRPCConfig contains gRPC server settings.
type GRPCConfig struct {
	Address string // gRPC server listen address (e.g., ":50051")
}

// AuthConfig contains authentication settings.
type AuthConfig struct {
	JWTSecret string // JWT signing secret
}

// Load loads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Database: DatabaseConfig{
			Path: getEnv("DB_PATH", "app.db"),
		},
		GRPC: GRPCConfig{
			Address: getEnv("GRPC_ADDRESS", ":50051"),
		},
		Auth: AuthConfig{
			JWTSecret: getEnv("JWT_SECRET", ""),
		},
	}

	// Validate critical settings
	if cfg.Auth.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is not set; required for production")
	}

	return cfg, nil
}

// LoadWithDefaults is like Load but uses a safe default for JWT_SECRET in development.
// WARNING: Only use in development! Use Load() in production.
func LoadWithDefaults() (*Config, error) {
	cfg := &Config{
		Database: DatabaseConfig{
			Path: getEnv("DB_PATH", "app.db"),
		},
		GRPC: GRPCConfig{
			Address: getEnv("GRPC_ADDRESS", ":50051"),
		},
		Auth: AuthConfig{
			JWTSecret: getEnv("JWT_SECRET", "dev-secret-change-me"),
		},
	}
	return cfg, nil
}

// getEnv retrieves an environment variable with a default fallback.
func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

// getEnvInt retrieves an environment variable as an integer with a default fallback.
func getEnvInt(key string, defaultVal int) (int, error) {
	if value, exists := os.LookupEnv(key); exists {
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("invalid integer for %s: %w", key, err)
		}
		return intVal, nil
	}
	return defaultVal, nil
}

// String returns a string representation of the config (sensitive values are masked).
func (c *Config) String() string {
	return fmt.Sprintf("Config{DB: %s, gRPC: %s, Auth: *** (masked) ***}", c.Database.Path, c.GRPC.Address)
}
