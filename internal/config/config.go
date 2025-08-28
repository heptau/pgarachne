package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost          string
	DBPort          int
	DBUser          string
	HTTPPort        string
	JWTSecret       string
	JWTExpiryHours  int
	AllowedOrigins  []string
	StaticFilesPath string
	LogLevel        string
	LogOutput       string
}

// Search paths for configuration
// 1. Explicitly provided path (flag)
// 2. Current directory: ./pgarachne.env
// 3. User config: $XDG_CONFIG_HOME/pgarachne/pgarachne.env (or ~/.config/...)
// 4. System config: /etc/pgarachne/pgarachne.env

func Load(configPath string) (*Config, error) {
	loadedFile := ""

	if configPath != "" {
		// 1. Explicit path
		if err := godotenv.Load(configPath); err != nil {
			return nil, fmt.Errorf("failed to load config file '%s': %w", configPath, err)
		}
		loadedFile = configPath
	} else {
		// Automatic search
		searchPaths := []string{
			"pgarachne.env", // Current dir
		}

		// User Config
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				configHome = filepath.Join(homeDir, ".config")
			}
		}
		if configHome != "" {
			searchPaths = append(searchPaths, filepath.Join(configHome, "pgarachne", "pgarachne.env"))
		}

		// System Config
		searchPaths = append(searchPaths, "/etc/pgarachne/pgarachne.env")

		// Try to load first existing
		for _, path := range searchPaths {
			if _, err := os.Stat(path); err == nil {
				if err := godotenv.Load(path); err == nil {
					loadedFile = path
					break
				}
			}
		}
	}

	if loadedFile != "" {
		fmt.Printf("Loaded configuration from: %s\n", loadedFile)
	} else {
		fmt.Println("No configuration file found in standard locations. Using environment variables only.")
	}

	cfg := &Config{}

	cfg.DBHost = os.Getenv("DB_HOST")
	cfg.DBUser = os.Getenv("DB_USER")
	cfg.HTTPPort = os.Getenv("HTTP_PORT")
	cfg.JWTSecret = os.Getenv("JWT_SECRET")

	cfg.LogLevel = os.Getenv("LOG_LEVEL")
	if cfg.LogLevel == "" {
		cfg.LogLevel = "INFO"
	}

	cfg.LogOutput = os.Getenv("LOG_OUTPUT")
	if cfg.LogOutput == "" {
		cfg.LogOutput = "stdout"
	}

	dbPortStr := os.Getenv("DB_PORT")
	if dbPortStr != "" {
		port, err := strconv.Atoi(dbPortStr)
		if err != nil {
			return nil, fmt.Errorf("invalid DB_PORT value: '%s', must be an integer", dbPortStr)
		}
		cfg.DBPort = port
	}

	jwtExpiryStr := os.Getenv("JWT_EXPIRY_HOURS")
	if jwtExpiryStr != "" {
		hours, err := strconv.Atoi(jwtExpiryStr)
		if err != nil {
			return nil, fmt.Errorf("invalid JWT_EXPIRY_HOURS value: '%s', must be an integer", jwtExpiryStr)
		}
		cfg.JWTExpiryHours = hours
	} else {
		cfg.JWTExpiryHours = 8 // Default
	}

	if cfg.HTTPPort == "" {
		cfg.HTTPPort = "8080"
	}

	allowedOriginsStr := os.Getenv("ALLOWED_ORIGINS")
	if allowedOriginsStr != "" {
		origins := strings.Split(allowedOriginsStr, ",")
		cfg.AllowedOrigins = make([]string, 0, len(origins))
		for _, origin := range origins {
			trimmedOrigin := strings.TrimSpace(origin)
			if trimmedOrigin != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, trimmedOrigin)
			}
		}
	}
	if len(cfg.AllowedOrigins) == 0 {
		cfg.AllowedOrigins = []string{"*"}
	}

	staticPath := os.Getenv("STATIC_FILES_PATH")
	if staticPath != "" {
		absPath, err := filepath.Abs(staticPath)
		if err != nil {
			return nil, fmt.Errorf("could not resolve absolute path for STATIC_FILES_PATH='%s': %w", staticPath, err)
		}
		info, err := os.Stat(absPath)
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("FATAL: The resolved static files path does not exist: %s", absPath)
		}
		if err != nil {
			return nil, fmt.Errorf("FATAL: Error checking static files path: %w", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("FATAL: The resolved static files path is not a directory: %s", absPath)
		}
		cfg.StaticFilesPath = absPath
	}

	if cfg.DBHost == "" || cfg.DBUser == "" || cfg.DBPort == 0 {
		return nil, fmt.Errorf("missing required database environment variables: DB_HOST, DB_USER, DB_PORT")
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("jwt_secret not set in config (environment variable JWT_SECRET)")
	}

	return cfg, nil
}
