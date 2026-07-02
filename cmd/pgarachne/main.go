package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/heptau/pgarachne/internal/config"
	"github.com/heptau/pgarachne/internal/daemon"
	"github.com/heptau/pgarachne/internal/server"
	"github.com/heptau/pgarachne/internal/version"
)

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

	// Handle Daemon commands first — each function calls os.Exit internally,
	// but explicit returns make the flow clear and prevent accidental fallthrough
	// when Stop() and Start() are both set.
	if *stopDaemon {
		daemon.Stop()
		return
	}

	if *startDaemon {
		daemon.Start()
		return
	}

	if *showHelp {
		flag.Usage()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("PgArachne %s\n", version.Full())
		return
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	handlerOptions := &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
	}

	fileLogging := cfg.LogOutput != "stdout"

	if fileLogging {
		// All structured logging goes to the log file.
		// Only brief human-readable messages are printed to the console.
		file, err := os.OpenFile(cfg.LogOutput, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open log file %q: %v\n", cfg.LogOutput, err)
			os.Exit(1)
		}
		defer file.Close()

		slog.SetDefault(slog.New(slog.NewJSONHandler(file, handlerOptions)))

		fmt.Printf("PgArachne %s starting. Logging to %s\n", version.Full(), cfg.LogOutput)
	} else {
		// No log file configured — write everything to stdout.
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, handlerOptions)))
	}

	slog.Info("PgArachne starting",
		"version", version.Version,
		"config_file", *configPath,
		"log_output", cfg.LogOutput,
		"log_level", cfg.LogLevel,
	)

	// Initialize and run server
	srv := server.New(cfg)
	if err := srv.Run(); err != nil {
		slog.Error("Server failed", "error", err)
		if fileLogging {
			fmt.Fprintf(os.Stderr, "PgArachne stopped with error: %v\n", err)
		}
		os.Exit(1)
	}

	slog.Info("PgArachne stopped")
	if fileLogging {
		fmt.Println("PgArachne stopped.")
	}
}
