//go:build !windows

package daemon

import (
	"path/filepath"
	"testing"
)

func TestPidFilePath_UsesCustomEnv(t *testing.T) {
	t.Setenv("PID_FILE", "/tmp/custom-pgarachne.pid")

	got := pidFilePath()
	if got != "/tmp/custom-pgarachne.pid" {
		t.Fatalf("pidFilePath() = %q, want %q", got, "/tmp/custom-pgarachne.pid")
	}
}

func TestPidFilePath_Default(t *testing.T) {
	t.Setenv("PID_FILE", "")

	got := pidFilePath()
	if filepath.Base(got) != "pgarachne.pid" {
		t.Fatalf("pidFilePath() = %q, expected filename pgarachne.pid", got)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("pidFilePath() = %q, expected absolute path", got)
	}
}
