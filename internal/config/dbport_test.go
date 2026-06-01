package config

import "testing"

func TestDBPortValidation(t *testing.T) {
	setRequiredTestEnv(t)
	valid := []struct {
		name string
		val  string
	}{
		{"min", "1"},
		{"standard postgres", "5432"},
		{"max", "65535"},
	}
	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DB_PORT", tt.val)
			cfg, err := Load("")
			if err != nil {
				t.Fatalf("Load with DB_PORT=%s: unexpected error: %v", tt.val, err)
			}
			if cfg.DBPort == 0 {
				t.Errorf("DBPort not set")
			}
		})
	}

	invalid := []struct {
		name string
		val  string
	}{
		{"zero", "0"},
		{"negative", "-1"},
		{"too large", "65536"},
		{"non-numeric", "abc"},
		{"float", "54.32"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DB_PORT", tt.val)
			if _, err := Load(""); err == nil {
				t.Fatalf("Load with DB_PORT=%s: expected error, got nil", tt.val)
			}
		})
	}
}

func TestDBPortMissingFails(t *testing.T) {
	setRequiredTestEnv(t)
	t.Setenv("DB_PORT", "")
	if _, err := Load(""); err == nil {
		t.Fatal("expected error when DB_PORT is unset, got nil")
	}
}
