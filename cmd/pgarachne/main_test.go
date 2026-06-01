package main

import (
	"log/slog"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		in   string
		want slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"INFO", slog.LevelInfo},
		{"debug", slog.LevelDebug},
		{"  warn  ", slog.LevelWarn},
		{"", slog.LevelInfo},
		{"VERBOSE", slog.LevelInfo},
	}
	for _, tt := range tests {
		if got := parseLogLevel(tt.in); got != tt.want {
			t.Errorf("parseLogLevel(%q) = %v; want %v", tt.in, got, tt.want)
		}
	}
}
