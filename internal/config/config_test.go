package config

import (
	"os"
	"testing"
)

func TestNoHardcodedDevKeyDefault(t *testing.T) {
	os.Unsetenv("JWT_SECRET")

	cfg := &Config{}
	setDefaults(cfg)

	if cfg.Auth.DefaultKey == "dev-mode-key-change-me" {
		t.Error("DefaultKey must not be 'dev-mode-key-change-me' - use env var REQUIEM_MASTER_KEY")
	}

	if cfg.Auth.DefaultKey != "" {
		t.Errorf("DefaultKey should be empty when not configured, got: %q", cfg.Auth.DefaultKey)
	}
}

func TestJWTSecretNotPredictable(t *testing.T) {
	os.Unsetenv("JWT_SECRET")

	cfg := &Config{}
	setDefaults(cfg)

	if cfg.Auth.JWTSecret == "requiem-dev-jwt-secret-change-me" {
		t.Error("JWT secret must not use the old predictable default")
	}
}

func TestJWTSecretFromEnv(t *testing.T) {
	expectedSecret := "my-test-jwt-secret-12345"
	os.Setenv("JWT_SECRET", expectedSecret)
	defer os.Unsetenv("JWT_SECRET")

	cfg := &Config{}
	setDefaults(cfg)

	if cfg.Auth.JWTSecret != expectedSecret {
		t.Errorf("expected JWT secret from env, got: %q", cfg.Auth.JWTSecret)
	}
}

func TestServerBindLocalhostByDefault(t *testing.T) {
	cfg := &Config{}
	setDefaults(cfg)

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host should default to 127.0.0.1 for security, got: %q", cfg.Server.Host)
	}
}
