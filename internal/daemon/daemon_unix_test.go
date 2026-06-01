//go:build !windows

package daemon

import (
	"os"
	"path/filepath"
	"strconv"
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

func TestIsRunning(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pgarachne.pid")
	t.Setenv("PID_FILE", pidFile)

	if isRunning() {
		t.Error("isRunning() = true with no PID file; want false")
	}

	if err := os.WriteFile(pidFile, []byte("not-a-pid"), 0600); err != nil {
		t.Fatalf("write pid file: %v", err)
	}
	if isRunning() {
		t.Error("isRunning() = true with garbage PID file content; want false")
	}

	// Our own PID is guaranteed to be alive and signalable.
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatalf("write pid file: %v", err)
	}
	if !isRunning() {
		t.Error("isRunning() = false for the test's own live PID; want true")
	}

	// A PID far above any real pid_max: FindProcess succeeds on Unix, but
	// signal 0 fails because the process does not exist.
	if err := os.WriteFile(pidFile, []byte("999999999"), 0600); err != nil {
		t.Fatalf("write pid file: %v", err)
	}
	if isRunning() {
		t.Error("isRunning() = true for a nonexistent PID; want false")
	}
}
