//go:build windows

package daemon

import (
	"fmt"
	"os"
)

// Start is a stub for Windows.
func Start() {
	fmt.Println("Daemon mode is not supported on Windows.")
	os.Exit(1)
}

// Stop is a stub for Windows.
func Stop() {
	fmt.Println("Daemon mode is not supported on Windows.")
	os.Exit(1)
}
