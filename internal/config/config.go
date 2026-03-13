package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"log/slog"
)

type Config struct {
	DBHost            string
	DBPort            int
	DBUser            string
	DBSSLMode         string
	DBSSLRootCert     string
	DBSSLCert         string
	DBSSLKey          string
	HTTPPort          string
	JWTSecret         string
	JWTExpiryHours    int
	AllowedOrigins    []string
	StaticFilesPath   string
	LogLevel          string
	LogOutput         string
	LoginRateLimit    int
	LoginRateWindow   time.Duration
	TrustedProxies    []string
	MaxRequestBytes   int64
	MetricsEnabled    bool
	MetricsListenAddr string
	SSEMaxChannels    int
	SSEHeartbeat      time.Duration
	SSEIdleTimeout    time.Duration
	SSEMaxClients     int
	SSEClientBuffer   int
	SSESendTimeout    time.Duration
	// APIPrefix is the first path segment for all database endpoints.
	// Defaults to "db", giving routes like /db/:database/jsonrpc and /db/:database/sse.
	// Set to "api" for backward-compatible paths, or any other value for custom deployments.
	APIPrefix string
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
		slog.Info("Loaded configuration", "config_file", loadedFile)
	} else {
		slog.Info("No configuration file found in standard locations. Using environment variables only.")
	}

	cfg := &Config{}

	cfg.DBHost = os.Getenv("DB_HOST")
	cfg.DBUser = os.Getenv("DB_USER")
	cfg.HTTPPort = os.Getenv("HTTP_PORT")
	cfg.JWTSecret = os.Getenv("JWT_SECRET")

	cfg.DBSSLMode = os.Getenv("DB_SSLMODE")
	if cfg.DBSSLMode == "" {
		cfg.DBSSLMode = "disable"
	}
	cfg.DBSSLRootCert = os.Getenv("DB_SSLROOTCERT")
	cfg.DBSSLCert = os.Getenv("DB_SSLCERT")
	cfg.DBSSLKey = os.Getenv("DB_SSLKEY")

	cfg.LogLevel = os.Getenv("LOG_LEVEL")
	if cfg.LogLevel == "" {
		cfg.LogLevel = "INFO"
	}

	cfg.LogOutput = os.Getenv("LOG_OUTPUT")
	if cfg.LogOutput == "" {
		cfg.LogOutput = "stdout"
	}

	cfg.LoginRateLimit = 5
	if limitStr := os.Getenv("LOGIN_RATE_LIMIT"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return nil, fmt.Errorf("invalid LOGIN_RATE_LIMIT value: '%s', must be an integer", limitStr)
		}
		if limit < 0 {
			return nil, fmt.Errorf("invalid LOGIN_RATE_LIMIT value: '%s', must be >= 0", limitStr)
		}
		cfg.LoginRateLimit = limit
	}

	cfg.LoginRateWindow = time.Minute
	if windowStr := os.Getenv("LOGIN_RATE_WINDOW"); windowStr != "" {
		window, err := time.ParseDuration(windowStr)
		if err != nil {
			return nil, fmt.Errorf("invalid LOGIN_RATE_WINDOW value: '%s', must be a duration like 1m or 30s", windowStr)
		}
		if window <= 0 {
			return nil, fmt.Errorf("invalid LOGIN_RATE_WINDOW value: '%s', must be > 0", windowStr)
		}
		cfg.LoginRateWindow = window
	}

	trustedProxiesStr := os.Getenv("TRUSTED_PROXIES")
	if trustedProxiesStr != "" {
		parts := strings.Split(trustedProxiesStr, ",")
		cfg.TrustedProxies = make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				cfg.TrustedProxies = append(cfg.TrustedProxies, trimmed)
			}
		}
	}

	cfg.MaxRequestBytes = 2 * 1024 * 1024 // 2MB default
	if maxBodyStr := os.Getenv("MAX_REQUEST_BYTES"); maxBodyStr != "" {
		value, err := strconv.ParseInt(maxBodyStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid MAX_REQUEST_BYTES value: '%s', must be an integer", maxBodyStr)
		}
		if value <= 0 {
			return nil, fmt.Errorf("invalid MAX_REQUEST_BYTES value: '%s', must be > 0", maxBodyStr)
		}
		cfg.MaxRequestBytes = value
	}

	cfg.MetricsEnabled = true
	if metricsEnabledStr := os.Getenv("METRICS_ENABLED"); metricsEnabledStr != "" {
		enabled, err := strconv.ParseBool(metricsEnabledStr)
		if err != nil {
			return nil, fmt.Errorf("invalid METRICS_ENABLED value: '%s', must be true/false", metricsEnabledStr)
		}
		cfg.MetricsEnabled = enabled
	}

	cfg.MetricsListenAddr = "127.0.0.1:9090"
	if metricsListenAddr := strings.TrimSpace(os.Getenv("METRICS_LISTEN_ADDR")); metricsListenAddr != "" {
		cfg.MetricsListenAddr = metricsListenAddr
	}

	cfg.SSEMaxChannels = 8
	if maxChannelsStr := os.Getenv("SSE_MAX_CHANNELS"); maxChannelsStr != "" {
		value, err := strconv.Atoi(maxChannelsStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SSE_MAX_CHANNELS value: '%s', must be an integer", maxChannelsStr)
		}
		if value <= 0 {
			return nil, fmt.Errorf("invalid SSE_MAX_CHANNELS value: '%s', must be > 0", maxChannelsStr)
		}
		cfg.SSEMaxChannels = value
	}

	cfg.SSEHeartbeat = 20 * time.Second
	if heartbeatStr := os.Getenv("SSE_HEARTBEAT"); heartbeatStr != "" {
		value, err := time.ParseDuration(heartbeatStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SSE_HEARTBEAT value: '%s', must be a duration like 20s", heartbeatStr)
		}
		if value <= 0 {
			return nil, fmt.Errorf("invalid SSE_HEARTBEAT value: '%s', must be > 0", heartbeatStr)
		}
		cfg.SSEHeartbeat = value
	}

	cfg.SSEIdleTimeout = 90 * time.Second
	if timeoutStr := os.Getenv("SSE_IDLE_TIMEOUT"); timeoutStr != "" {
		value, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SSE_IDLE_TIMEOUT value: '%s', must be a duration like 90s", timeoutStr)
		}
		if value <= 0 {
			return nil, fmt.Errorf("invalid SSE_IDLE_TIMEOUT value: '%s', must be > 0", timeoutStr)
		}
		cfg.SSEIdleTimeout = value
	}

	cfg.SSEMaxClients = 1000
	if maxClientsStr := os.Getenv("SSE_MAX_CLIENTS"); maxClientsStr != "" {
		value, err := strconv.Atoi(maxClientsStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SSE_MAX_CLIENTS value: '%s', must be an integer", maxClientsStr)
		}
		if value <= 0 {
			return nil, fmt.Errorf("invalid SSE_MAX_CLIENTS value: '%s', must be > 0", maxClientsStr)
		}
		cfg.SSEMaxClients = value
	}

	cfg.SSEClientBuffer = 64
	if bufferStr := os.Getenv("SSE_CLIENT_BUFFER"); bufferStr != "" {
		value, err := strconv.Atoi(bufferStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SSE_CLIENT_BUFFER value: '%s', must be an integer", bufferStr)
		}
		if value <= 0 {
			return nil, fmt.Errorf("invalid SSE_CLIENT_BUFFER value: '%s', must be > 0", bufferStr)
		}
		cfg.SSEClientBuffer = value
	}

	cfg.SSESendTimeout = 2 * time.Second
	if sendTimeoutStr := os.Getenv("SSE_SEND_TIMEOUT"); sendTimeoutStr != "" {
		value, err := time.ParseDuration(sendTimeoutStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SSE_SEND_TIMEOUT value: '%s', must be a duration like 2s", sendTimeoutStr)
		}
		if value <= 0 {
			return nil, fmt.Errorf("invalid SSE_SEND_TIMEOUT value: '%s', must be > 0", sendTimeoutStr)
		}
		cfg.SSESendTimeout = value
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

	if err := validateDBSSLConfig(cfg); err != nil {
		return nil, err
	}
	if err := validateMetricsConfig(cfg); err != nil {
		return nil, err
	}

	cfg.APIPrefix = strings.TrimSpace(os.Getenv("API_PREFIX"))
	if cfg.APIPrefix == "" {
		cfg.APIPrefix = "db"
	}
	if err := validateAPIPrefix(cfg.APIPrefix); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validateDBSSLConfig(cfg *Config) error {
	if cfg.DBSSLMode == "" {
		return fmt.Errorf("db sslmode is empty")
	}

	// If any cert path is provided, ensure it exists and is a file.
	for key, path := range map[string]string{
		"DB_SSLROOTCERT": cfg.DBSSLRootCert,
		"DB_SSLCERT":     cfg.DBSSLCert,
		"DB_SSLKEY":      cfg.DBSSLKey,
	} {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			return fmt.Errorf("%s file does not exist: %s", key, path)
		}
		if err != nil {
			return fmt.Errorf("error checking %s file: %w", key, err)
		}
		if info.IsDir() {
			return fmt.Errorf("%s path is a directory, expected file: %s", key, path)
		}
	}

	return nil
}

func validateMetricsConfig(cfg *Config) error {
	if !cfg.MetricsEnabled {
		return nil
	}
	if cfg.MetricsListenAddr == "" {
		return fmt.Errorf("metrics listen address is empty")
	}
	if _, err := net.ResolveTCPAddr("tcp", cfg.MetricsListenAddr); err != nil {
		return fmt.Errorf("invalid METRICS_LISTEN_ADDR value: '%s': %w", cfg.MetricsListenAddr, err)
	}
	return nil
}

// validateAPIPrefix checks that the prefix contains only URL-safe characters:
// letters, digits, hyphens, and underscores. Slashes are intentionally forbidden
// because the prefix occupies exactly one path segment.
func validateAPIPrefix(prefix string) error {
	if prefix == "" {
		return fmt.Errorf("API_PREFIX cannot be empty")
	}
	for _, r := range prefix {
		ok := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_'
		if !ok {
			return fmt.Errorf("invalid API_PREFIX value: '%s', only letters, digits, hyphens and underscores are allowed", prefix)
		}
	}
	return nil
}

// DBSSLParams returns connection parameters for SSL/TLS configuration.
// Values are included only when present to avoid overriding lib defaults.
func (c *Config) DBSSLParams() string {
	parts := []string{}
	if c.DBSSLMode != "" {
		parts = append(parts, fmt.Sprintf("sslmode=%s", c.DBSSLMode))
	}
	if c.DBSSLRootCert != "" {
		parts = append(parts, fmt.Sprintf("sslrootcert=%s", c.DBSSLRootCert))
	}
	if c.DBSSLCert != "" {
		parts = append(parts, fmt.Sprintf("sslcert=%s", c.DBSSLCert))
	}
	if c.DBSSLKey != "" {
		parts = append(parts, fmt.Sprintf("sslkey=%s", c.DBSSLKey))
	}
	return strings.Join(parts, " ")
}
