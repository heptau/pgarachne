//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

const PidFile = "/tmp/pgarachne.pid"

// Start launches the current executable in the background.
// It removes the "-start" flag from arguments to prevent recursive spawning.
func Start() {
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

	cmd := exec.Command(os.Args[0], args...)
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
	if err := os.WriteFile(PidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		fmt.Printf("Process started (PID %d), but failed to write PID file: %v\n", cmd.Process.Pid, err)
		// We don't exit here, the process is running.
	} else {
		fmt.Printf("PgArachne started in background with PID %d\n", cmd.Process.Pid)
	}

	os.Exit(0)
}

// Stop terminates the background process using the PID file.
func Stop() {
	pidData, err := os.ReadFile(PidFile)
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
		os.Remove(PidFile)
		os.Exit(1)
	}

	// Send SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Printf("Failed to stop process (PID %d): %v\n", pid, err)
		os.Exit(1)
	}

	// Wait a bit and check if it's gone?
	// For now, just assume it works and remove PID file.
	time.Sleep(100 * time.Millisecond)

	if err := os.Remove(PidFile); err != nil {
		fmt.Printf("Stopped process (PID %d), but failed to remove PID file: %v\n", pid, err)
	} else {
		fmt.Println("PgArachne stopped.")
	}

	os.Exit(0)
}

func isRunning() bool {
	pidData, err := os.ReadFile(PidFile)
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
