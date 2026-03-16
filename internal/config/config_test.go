package config

import (
	"os"
	"testing"
)

func mustSetenv(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("os.Setenv(%q): %v", key, err)
	}
}

func mustUnsetenv(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("os.Unsetenv(%q): %v", key, err)
	}
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	mustUnsetenv(t, "JWT_SECRET")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is missing")
	}
}

func TestLoad_ShortJWTSecret(t *testing.T) {
	mustSetenv(t, "JWT_SECRET", "tooshort")
	defer mustUnsetenv(t, "JWT_SECRET")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is too short")
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	mustSetenv(t, "JWT_SECRET", "a-very-long-secret-key-that-is-at-least-32-chars")
	defer mustUnsetenv(t, "JWT_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.JWTSecret == "" {
		t.Error("expected non-empty JWT secret")
	}
	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Port)
	}
	if cfg.JWTAccessTTLHours != 24 {
		t.Errorf("expected default TTL 24, got %d", cfg.JWTAccessTTLHours)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	mustSetenv(t, "JWT_SECRET", "a-very-long-secret-key-that-is-at-least-32-chars")
	mustSetenv(t, "PORT", "9090")
	mustSetenv(t, "ENV", "production")
	mustSetenv(t, "JWT_ACCESS_TTL_HOURS", "48")
	mustSetenv(t, "CORS_ALLOWED_ORIGINS", "https://example.com,https://api.example.com")
	defer func() {
		mustUnsetenv(t, "JWT_SECRET")
		mustUnsetenv(t, "PORT")
		mustUnsetenv(t, "ENV")
		mustUnsetenv(t, "JWT_ACCESS_TTL_HOURS")
		mustUnsetenv(t, "CORS_ALLOWED_ORIGINS")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Port)
	}
	if cfg.JWTAccessTTLHours != 48 {
		t.Errorf("expected TTL 48, got %d", cfg.JWTAccessTTLHours)
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("expected 2 origins, got %d", len(cfg.AllowedOrigins))
	}
}

func TestLoad_InvalidTTLDefaultsTo24(t *testing.T) {
	mustSetenv(t, "JWT_SECRET", "a-very-long-secret-key-that-is-at-least-32-chars")
	mustSetenv(t, "JWT_ACCESS_TTL_HOURS", "-5")
	defer func() {
		mustUnsetenv(t, "JWT_SECRET")
		mustUnsetenv(t, "JWT_ACCESS_TTL_HOURS")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.JWTAccessTTLHours != 24 {
		t.Errorf("expected TTL to default to 24, got %d", cfg.JWTAccessTTLHours)
	}
}

func TestIsProduction(t *testing.T) {
	mustSetenv(t, "JWT_SECRET", "a-very-long-secret-key-that-is-at-least-32-chars")
	defer mustUnsetenv(t, "JWT_SECRET")

	tests := []struct {
		env      string
		expected bool
	}{
		{"production", true},
		{"development", false},
		{"staging", false},
	}

	for _, tt := range tests {
		mustSetenv(t, "ENV", tt.env)
		cfg, _ := Load()
		if cfg.IsProduction() != tt.expected {
			t.Errorf("env=%s: expected IsProduction()=%v", tt.env, tt.expected)
		}
		mustUnsetenv(t, "ENV")
	}
}

func TestDSN(t *testing.T) {
	mustSetenv(t, "JWT_SECRET", "a-very-long-secret-key-that-is-at-least-32-chars")
	defer mustUnsetenv(t, "JWT_SECRET")

	cfg, _ := Load()
	dsn := cfg.DSN()
	if dsn == "" {
		t.Error("expected non-empty DSN")
	}
	// Should contain key components
	for _, part := range []string{"host=", "user=", "dbname=", "port="} {
		found := false
		for i := 0; i+len(part) <= len(dsn); i++ {
			if dsn[i:i+len(part)] == part {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("DSN missing %q: %s", part, dsn)
		}
	}
}
