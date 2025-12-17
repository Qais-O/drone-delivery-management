package config

import (
	"os"
	"testing"
)

func TestLoadWithDefaults_Succeeds(t *testing.T) {
	// Ensure envs are clean to use defaults
	os.Unsetenv("DB_PATH")
	os.Unsetenv("GRPC_ADDRESS")
	os.Unsetenv("JWT_SECRET")
	cfg, err := LoadWithDefaults()
	if err != nil {
		t.Fatalf("LoadWithDefaults: %v", err)
	}
	if cfg.GRPC.Address == "" || cfg.Database.Path == "" || cfg.Auth.JWTSecret == "" {
		t.Fatalf("unexpected empty defaults: %+v", cfg)
	}
}

func TestLoad_RequiresJWTSecret(t *testing.T) {
	// Clear JWT_SECRET ensures error
	os.Unsetenv("JWT_SECRET")
	// Other vars can be set or default
	t.Setenv("DB_PATH", "test.db")
	t.Setenv("GRPC_ADDRESS", ":1234")
	if _, err := Load(); err == nil {
		t.Fatalf("expected error when JWT_SECRET is not set")
	}
	// When set, it should succeed
	t.Setenv("JWT_SECRET", "x")
	if _, err := Load(); err != nil {
		t.Fatalf("Load with secret set: %v", err)
	}
}
