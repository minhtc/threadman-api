package config

import "testing"

func TestLoadDefaultsAndOrigins(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("LEADERBOARD_TIMEZONE", "UTC")
	t.Setenv("ALLOWED_ORIGINS", " https://example.com/,https://game.example.com ")
	t.Setenv("SESSION_SECRET_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want 8080", cfg.Port)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://example.com" {
		t.Fatalf("AllowedOrigins = %#v", cfg.AllowedOrigins)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SESSION_SECRET_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded without DATABASE_URL")
	}
}

func TestLoadAcceptsCapacitorOrigin(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("SESSION_SECRET_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	t.Setenv("ALLOWED_ORIGINS", "capacitor://localhost")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "capacitor://localhost" {
		t.Fatalf("AllowedOrigins = %#v, want capacitor origin", cfg.AllowedOrigins)
	}
}

func TestLoadAcceptsWildcardOrigin(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("SESSION_SECRET_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	t.Setenv("ALLOWED_ORIGINS", "*")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "*" {
		t.Fatalf("AllowedOrigins = %#v, want wildcard", cfg.AllowedOrigins)
	}
}

func TestLoadRejectsInvalidTrustedProxy(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("SESSION_SECRET_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	t.Setenv("TRUST_PROXY", "true")
	t.Setenv("TRUSTED_PROXIES", "not-an-ip")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted an invalid trusted proxy")
	}
}
