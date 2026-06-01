package config

import (
	"testing"
	"time"
)

func TestJWTDefaults(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("JWT_ISSUER", "")
	t.Setenv("JWT_AUDIENCE", "")
	t.Setenv("JWT_LEEWAY", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.JWTIssuer != "" {
		t.Errorf("JWTIssuer default = %q; want empty", cfg.JWTIssuer)
	}
	if cfg.JWTAudience != "" {
		t.Errorf("JWTAudience default = %q; want empty", cfg.JWTAudience)
	}
	if cfg.JWTLeeway != 0 {
		t.Errorf("JWTLeeway default = %v; want 0 (use auth.DefaultClockSkew)", cfg.JWTLeeway)
	}
}

func TestJWTOverride(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("JWT_ISSUER", "pgarachne.example.com")
	t.Setenv("JWT_AUDIENCE", "pgarachne-api")
	t.Setenv("JWT_LEEWAY", "45s")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.JWTIssuer != "pgarachne.example.com" {
		t.Errorf("JWTIssuer = %q; want pgarachne.example.com", cfg.JWTIssuer)
	}
	if cfg.JWTAudience != "pgarachne-api" {
		t.Errorf("JWTAudience = %q; want pgarachne-api", cfg.JWTAudience)
	}
	if cfg.JWTLeeway != 45*time.Second {
		t.Errorf("JWTLeeway = %v; want 45s", cfg.JWTLeeway)
	}
}

func TestJWTLeewayRejectsInvalid(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("JWT_LEEWAY", "forever")
	if _, err := Load(""); err == nil {
		t.Error("expected error for invalid JWT_LEEWAY, got nil")
	}

	t.Setenv("JWT_LEEWAY", "-5s")
	if _, err := Load(""); err == nil {
		t.Error("expected error for negative JWT_LEEWAY, got nil")
	}
}
