//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func pidFilePath() string {
	if custom := strings.TrimSpace(os.Getenv("PID_FILE")); custom != "" {
		return custom
	}

	cacheDir, err := os.UserCacheDir()
	if err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "pgarachne", "pgarachne.pid")
	}

	return filepath.Join(os.TempDir(), "pgarachne.pid")
}

// Start launches the current executable in the background.
// It removes the "-start" flag from arguments to prevent recursive spawning.
func Start() {
	pidFile := pidFilePath()

	if isRunning() {
		fmt.Println("PgArachne is already running.")
		os.Exit(1)
	}

	// Prepare arguments for the child process
	args := []string{}
	for _, arg := range os.Args[1:] {
		if arg != "-start" && arg != "--start" {
			args = append(args, arg)
		}
	}

	// Relaunching our own binary with the user's own arguments (minus -start)
	// is the point of daemon mode — there is no untrusted input here.
	cmd := exec.Command(os.Args[0], args...) //nolint:gosec // G204: re-exec of self
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Detach from terminal
	}

	// Unlink stdio to ensure full detachment
	// If logging is configured to file, the child will re-open it.
	// We can't easily redirect stdout/stderr here without knowing the config,
	// but strictly speaking, a daemon shouldn't write to the parent's terminal.
	// For simplicity, we let them go to /dev/null by default (exec behavior if not set).
	// Actually, exec.Command inherits stdio by default if not set.
	// To truly detach, we should set them to nil or file.
	// Let's set them to nil so it doesn't hang on terminal I/O.
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start background process: %v\n", err)
		os.Exit(1)
	}

	// Write PID file
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		fmt.Printf("Process started (PID %d), but failed to create PID directory: %v\n", cmd.Process.Pid, err)
		os.Exit(0)
	}
	// pidFile comes from pidFilePath(), which is either the operator's own
	// PID_FILE env var or an OS-provided cache/temp directory — not
	// attacker-controlled input.
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0600); err != nil { //nolint:gosec // G703: operator-controlled path
		fmt.Printf("Process started (PID %d), but failed to write PID file: %v\n", cmd.Process.Pid, err)
		// We don't exit here, the process is running.
	} else {
		fmt.Printf("PgArachne started in background with PID %d\n", cmd.Process.Pid)
	}

	os.Exit(0)
}

// Stop terminates the background process using the PID file.
func Stop() {
	pidFile := pidFilePath()
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("PgArachne is not running (PID file not found).")
			os.Exit(1)
		}
		fmt.Printf("Failed to read PID file: %v\n", err)
		os.Exit(1)
	}

	pid, err := strconv.Atoi(string(pidData))
	if err != nil {
		fmt.Printf("Invalid PID in file: %v\n", err)
		os.Exit(1)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("Failed to find process: %v\n", err)
		// Try to remove PID file anyway?
		os.Remove(pidFile)
		os.Exit(1)
	}

	// Send SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Printf("Failed to stop process (PID %d): %v\n", pid, err)
		os.Exit(1)
	}

	// Poll until the process is gone or a 10-second deadline is exceeded.
	// PgArachne's graceful-shutdown timeout is 5 s, so 10 s is ample.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if process.Signal(syscall.Signal(0)) != nil {
			break // process is gone
		}
	}

	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Stopped process (PID %d), but failed to remove PID file: %v\n", pid, err)
	} else {
		fmt.Println("PgArachne stopped.")
	}

	os.Exit(0)
}

func isRunning() bool {
	pidData, err := os.ReadFile(pidFilePath())
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(string(pidData))
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds, we need to send signal 0 to check existence
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return process.Signal(syscall.Signal(0)) == nil
}
