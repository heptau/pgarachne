package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/yourusername/pgarachne/internal/config"
	"github.com/yourusername/pgarachne/internal/daemon"
	"github.com/yourusername/pgarachne/internal/server"
)

var Version = "0.0.0"

func parseLogLevel(level string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file. If empty, searches standard locations.")
	showVersion := flag.Bool("version", false, "Show version and exit")
	showHelp := flag.Bool("help", false, "Show help message and exit")
	startDaemon := flag.Bool("start", false, "Start the server in the background")
	stopDaemon := flag.Bool("stop", false, "Stop the background server")

	flag.Parse()

	// Handle Daemon commands first
	if *stopDaemon {
		daemon.Stop()
	}

	if *startDaemon {
		daemon.Start()
	}

	if *showHelp {
		flag.Usage()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("PgArachne version %s\n", Version)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	logLevel := parseLogLevel(cfg.LogLevel)

	var logHandler slog.Handler
	var logFile *os.File
	handlerOptions := &slog.HandlerOptions{
		Level: logLevel,
	}

	if cfg.LogOutput != "stdout" {
		file, err := os.OpenFile(cfg.LogOutput, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open log file %q: %v\n", cfg.LogOutput, err)
			os.Exit(1)
		}
		logFile = file
		logHandler = slog.NewJSONHandler(file, handlerOptions)
	} else {
		logHandler = slog.NewJSONHandler(os.Stdout, handlerOptions)
	}

	logger := slog.New(logHandler)
	slog.SetDefault(logger)
	if logFile != nil {
		defer logFile.Close()
	}

	slog.Info("PgArachne starting", "version", Version)
	slog.Info("Configuration loaded successfully", "config_file", *configPath, "log_output", cfg.LogOutput, "log_level", cfg.LogLevel)
	if cfg.LogOutput != "stdout" {
		fmt.Printf("PgArachne version %s started. Logging to %s\n", Version, cfg.LogOutput)
	}

	// Initialize and run server
	srv := server.New(cfg)
	if err := srv.Run(); err != nil {
		slog.Error("Server failed", "error", err)
		// Clean up PID file if we are the daemon process is implicit,
		// but since we daemonize by re-executing, the child is just a normal process now.
		// A proper daemon manager might catch signals and remove PID, but our daemon.Stop() handles removal.
		// If it crashes, PID file stays (stale). This is typical for simple types.
		os.Exit(1)
	}
	if cfg.LogOutput != "stdout" {
		fmt.Println("PgArachne stopped.")
	}
}
