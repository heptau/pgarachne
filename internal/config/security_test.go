package config

import "testing"

func TestJWTSecretTooShortRejected(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("JWT_SECRET", "short_secret")

	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for JWT_SECRET shorter than 32 bytes, got nil")
	}
	if !contains(err.Error(), "JWT_SECRET") {
		t.Errorf("error = %q; want substring JWT_SECRET", err.Error())
	}
}

func TestJWTSecretPlaceholderRejected(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("JWT_SECRET", jwtSecretPlaceholder)

	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for placeholder JWT_SECRET, got nil")
	}
	if !contains(err.Error(), "placeholder") {
		t.Errorf("error = %q; want substring placeholder", err.Error())
	}
}

func TestDBSSLModeDefaultsToRequire(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("DB_SSLMODE", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DBSSLMode != "require" {
		t.Errorf("DBSSLMode default = %q; want require", cfg.DBSSLMode)
	}
}

func TestDBSSLModeExplicitDisableAccepted(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("DB_SSLMODE", "disable")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DBSSLMode != "disable" {
		t.Errorf("DBSSLMode = %q; want disable", cfg.DBSSLMode)
	}
}

func TestAllowedOriginsDefaultEmpty(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("ALLOWED_ORIGINS", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.AllowedOrigins) != 0 {
		t.Errorf("AllowedOrigins default = %v; want empty (same-origin only)", cfg.AllowedOrigins)
	}
}

func TestAllowedOriginsExplicitWildcard(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("ALLOWED_ORIGINS", "*")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "*" {
		t.Errorf("AllowedOrigins = %v; want [*]", cfg.AllowedOrigins)
	}
}

func TestLoginRateLimitPerIPDefault(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("LOGIN_RATE_LIMIT", "5")
	t.Setenv("LOGIN_RATE_LIMIT_PER_IP", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.LoginRateLimitPerIP != 25 {
		t.Errorf("LoginRateLimitPerIP default = %d; want 25 (5x LOGIN_RATE_LIMIT)", cfg.LoginRateLimitPerIP)
	}
}

func TestLoginRateLimitPerIPOverrideAndInvalid(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("LOGIN_RATE_LIMIT_PER_IP", "100")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.LoginRateLimitPerIP != 100 {
		t.Errorf("LoginRateLimitPerIP = %d; want 100", cfg.LoginRateLimitPerIP)
	}

	t.Setenv("LOGIN_RATE_LIMIT_PER_IP", "-1")
	if _, err := Load(""); err == nil {
		t.Error("expected error for negative LOGIN_RATE_LIMIT_PER_IP, got nil")
	}
}

func TestMCPSQLErrorDetailParsing(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("MCP_SQL_ERROR_DETAIL", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.MCPSQLErrorDetail {
		t.Error("MCPSQLErrorDetail default = true; want false")
	}

	t.Setenv("MCP_SQL_ERROR_DETAIL", "true")
	cfg, err = Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.MCPSQLErrorDetail {
		t.Error("MCPSQLErrorDetail = false; want true")
	}

	t.Setenv("MCP_SQL_ERROR_DETAIL", "maybe")
	if _, err := Load(""); err == nil {
		t.Error("expected error for invalid MCP_SQL_ERROR_DETAIL, got nil")
	}
}

func TestDirectPoolLimitParsing(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("DIRECT_POOL_LIMIT", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DirectPoolLimit != 1000 {
		t.Errorf("DirectPoolLimit default = %d; want 1000", cfg.DirectPoolLimit)
	}

	t.Setenv("DIRECT_POOL_LIMIT", "200")
	cfg, err = Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DirectPoolLimit != 200 {
		t.Errorf("DirectPoolLimit = %d; want 200", cfg.DirectPoolLimit)
	}

	t.Setenv("DIRECT_POOL_LIMIT", "0")
	if _, err := Load(""); err == nil {
		t.Error("expected error for DIRECT_POOL_LIMIT=0, got nil")
	}
}
