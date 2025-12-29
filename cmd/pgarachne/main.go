package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/yourusername/pgarachne/internal/config"
	"github.com/yourusername/pgarachne/internal/daemon"
	"github.com/yourusername/pgarachne/internal/server"
)

const Version = "1.0.1"

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

	// Setup temporary logger for startup
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Re-configure logging based on config
	var logLevel slog.Level
	switch cfg.LogLevel {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "WARN":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	var logHandler slog.Handler
	handlerOptions := &slog.HandlerOptions{
		Level: logLevel,
	}

	if cfg.LogOutput != "stdout" {
		file, err := os.OpenFile(cfg.LogOutput, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			slog.Error("Failed to open log file", "file", cfg.LogOutput, "error", err)
			os.Exit(1)
		}
		// Note: file is valid here, but we don't strictly close it as main exits immediately after,
		// or server runs until interrupt. In a long running service, this is usually acceptable for the main logger.
		logHandler = slog.NewJSONHandler(file, handlerOptions)
	} else {
		logHandler = slog.NewJSONHandler(os.Stdout, handlerOptions)
	}

	logger = slog.New(logHandler)
	slog.SetDefault(logger)

	slog.Info("Configuration loaded successfully", "config_file", *configPath)

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
}
