package config

import (
	"strings"
	"testing"
	"time"
)

func TestQuoteConninfoValue(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"simple", "'simple'"},
		{"has space", "'has space'"},
		{"has'single", "'has\\'single'"},
		{`has\backslash`, `'has\\backslash'`},
		{"", "''"},
	}

	for _, tt := range tests {
		if got := QuoteConninfoValue(tt.in); got != tt.want {
			t.Fatalf("QuoteConninfoValue(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	out := QuoteConninfoValue("x")
	if !strings.HasPrefix(out, "'") || !strings.HasSuffix(out, "'") {
		t.Fatalf("QuoteConninfoValue must always return single-quoted values, got %q", out)
	}
}

// TestDBSSLParamsQuoting verifies that SSL cert paths with spaces or special
// characters are properly quoted in the connection string.
func TestDBSSLParamsQuoting(t *testing.T) {
	cfg := &Config{
		DBHost:            "localhost",
		DBPort:            5432,
		DBUser:            "pg",
		HTTPPort:          "8080",
		JWTSecret:         "secret",
		JWTExpiryHours:    8,
		AllowedOrigins:    []string{"*"},
		DBMaxOpenConns:    25,
		DBMaxIdleConns:    5,
		DBConnMaxLifetime: 30 * time.Minute,
		DBConnMaxIdleTime: 5 * time.Minute,
		DBSSLMode:         "verify-full",
		DBSSLRootCert:     "/path/with space/root.crt",
		DBSSLCert:         "/path/with space/client.crt",
		DBSSLKey:          "/path/with space/client.key",
	}
	params := cfg.DBSSLParams()

	if !strings.Contains(params, "sslmode='verify-full'") {
		t.Errorf("DBSSLParams() = %q; want sslmode to be single-quoted", params)
	}
	if !strings.Contains(params, "sslrootcert='/path/with space/root.crt'") {
		t.Errorf("DBSSLParams() = %q; want sslrootcert path to be single-quoted", params)
	}
	if !strings.Contains(params, "sslcert='/path/with space/client.crt'") {
		t.Errorf("DBSSLParams() = %q; want sslcert path to be single-quoted", params)
	}
	if !strings.Contains(params, "sslkey='/path/with space/client.key'") {
		t.Errorf("DBSSLParams() = %q; want sslkey path to be single-quoted", params)
	}
}

// BenchmarkQuoteConninfoValue measures the cost of escaping a value
// for use in a libpq conninfo string. This runs on every JSON-RPC and
// MCP dispatch (the SET LOCAL app.api_prefix path), so the cost is on
// the per-request hot path.
func BenchmarkQuoteConninfoValue(b *testing.B) {
	cases := []struct {
		name  string
		value string
	}{
		{"plain", "db"},
		{"with_quote", "ab'cd"},
		{"with_backslash", `ab\cd`},
		{"long", strings.Repeat("a", 128)},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = QuoteConninfoValue(tc.value)
			}
		})
	}
}
