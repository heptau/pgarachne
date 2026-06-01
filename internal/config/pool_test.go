package config

import (
	"testing"
	"time"
)

func TestPoolLimitsDefaults(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("DB_MAX_OPEN_CONNS", "")
	t.Setenv("DB_MAX_IDLE_CONNS", "")
	t.Setenv("DB_CONN_MAX_LIFETIME", "")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DBMaxOpenConns != 25 {
		t.Errorf("DBMaxOpenConns default = %d; want 25", cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns != 5 {
		t.Errorf("DBMaxIdleConns default = %d; want 5", cfg.DBMaxIdleConns)
	}
	if cfg.DBConnMaxLifetime != 30*time.Minute {
		t.Errorf("DBConnMaxLifetime default = %v; want 30m", cfg.DBConnMaxLifetime)
	}
	if cfg.DBConnMaxIdleTime != 5*time.Minute {
		t.Errorf("DBConnMaxIdleTime default = %v; want 5m", cfg.DBConnMaxIdleTime)
	}
}

func TestPoolLimitsOverride(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("DB_MAX_OPEN_CONNS", "10")
	t.Setenv("DB_MAX_IDLE_CONNS", "4")
	t.Setenv("DB_CONN_MAX_LIFETIME", "15m")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "2m")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DBMaxOpenConns != 10 {
		t.Errorf("DBMaxOpenConns = %d; want 10", cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns != 4 {
		t.Errorf("DBMaxIdleConns = %d; want 4", cfg.DBMaxIdleConns)
	}
	if cfg.DBConnMaxLifetime != 15*time.Minute {
		t.Errorf("DBConnMaxLifetime = %v; want 15m", cfg.DBConnMaxLifetime)
	}
	if cfg.DBConnMaxIdleTime != 2*time.Minute {
		t.Errorf("DBConnMaxIdleTime = %v; want 2m", cfg.DBConnMaxIdleTime)
	}
}

func TestPoolLimitsRejectInvalid(t *testing.T) {
	setRequiredTestEnv(t)
	tests := []struct {
		name    string
		envVar  string
		envVal  string
		wantErr string
	}{
		{"non-numeric open", "DB_MAX_OPEN_CONNS", "abc", "DB_MAX_OPEN_CONNS"},
		{"zero open", "DB_MAX_OPEN_CONNS", "0", "DB_MAX_OPEN_CONNS"},
		{"negative open", "DB_MAX_OPEN_CONNS", "-1", "DB_MAX_OPEN_CONNS"},
		{"non-numeric idle", "DB_MAX_IDLE_CONNS", "abc", "DB_MAX_IDLE_CONNS"},
		{"zero idle", "DB_MAX_IDLE_CONNS", "0", "DB_MAX_IDLE_CONNS"},
		{"bad lifetime", "DB_CONN_MAX_LIFETIME", "forever", "DB_CONN_MAX_LIFETIME"},
		{"zero lifetime", "DB_CONN_MAX_LIFETIME", "0s", "DB_CONN_MAX_LIFETIME"},
		{"bad idle time", "DB_CONN_MAX_IDLE_TIME", "soon", "DB_CONN_MAX_IDLE_TIME"},
	}

	// Separate check: idle > open should be rejected even when both are valid integers.
	t.Run("idle exceeds open", func(t *testing.T) {
		for _, v := range []string{"DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME", "DB_CONN_MAX_IDLE_TIME"} {
			t.Setenv(v, "")
		}
		t.Setenv("DB_MAX_OPEN_CONNS", "5")
		t.Setenv("DB_MAX_IDLE_CONNS", "10")
		_, err := Load("")
		if err == nil {
			t.Fatal("expected error when DB_MAX_IDLE_CONNS > DB_MAX_OPEN_CONNS; got nil")
		}
		if !contains(err.Error(), "DB_MAX_IDLE_CONNS") {
			t.Errorf("error = %q; want substring DB_MAX_IDLE_CONNS", err.Error())
		}
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, v := range []string{"DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME", "DB_CONN_MAX_IDLE_TIME"} {
				t.Setenv(v, "")
			}
			t.Setenv(tt.envVar, tt.envVal)
			_, err := Load("")
			if err == nil {
				t.Fatalf("expected error containing %q; got nil", tt.wantErr)
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q; want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
