package config

import (
	"strings"
	"testing"
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
