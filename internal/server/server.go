package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yourusername/pgarachne/internal/config"
	"github.com/yourusername/pgarachne/internal/database"
)

type Server struct {
	Cfg    *config.Config
	sseHub *sseHub
}

var (
	// Postgres identifier (unquoted): starts with letter or underscore, then letters/digits/underscore/$
	pgIdentPattern = `[A-Za-z_][A-Za-z0-9_$]*`
	// Postgres quoted identifier: "..." with doubled quotes for escaping
	pgQuotedIdentPattern = `"([^"]|"")+"`

	pgIdentRe       = regexp.MustCompile(`^` + pgIdentPattern + `$`)
	pgQuotedIdentRe = regexp.MustCompile(`^` + pgQuotedIdentPattern + `$`)
	// schema.function where each part is quoted or unquoted
	pgFunctionRe = regexp.MustCompile(`^(` + pgIdentPattern + `|` + pgQuotedIdentPattern + `)\.(` + pgIdentPattern + `|` + pgQuotedIdentPattern + `)$`)

	loginAttemptLimiter *loginLimiter
)

func New(cfg *config.Config) *Server {
	initLoginLimiter(cfg)
	return &Server{Cfg: cfg, sseHub: newSSEHub(cfg)}
}

func (s *Server) Run() error {
	gin.SetMode(gin.ReleaseMode)
	router := s.buildRouter()
	metricsRouter := s.buildMetricsRouter()

	slog.Info("Starting PgArachne server", "port", s.Cfg.HTTPPort)

	srv := &http.Server{
		Addr:              ":" + s.Cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	metricsSrv := &http.Server{
		Addr:              s.Cfg.MetricsListenAddr,
		Handler:           metricsRouter,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       2 * time.Minute,
		WriteTimeout:      15 * time.Second,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "error", err)
			// If server fails to start, we must exit, but we are in a goroutine.
			// Ideally we communicate back, but os.Exit is acceptable for fatal startup error.
			// However, Run() should probably return error.
			// Let's rely on the main function handling, but here we can't easily bubble up error
			// without a channel. For simplicity in this structure:
			// We log and let the shutdown logic finish (or if start failed immediately).
		}
	}()
	go func() {
		if !s.Cfg.MetricsEnabled {
			slog.Info("Metrics endpoint disabled")
			return
		}
		slog.Info("Starting metrics server", "listen_addr", s.Cfg.MetricsListenAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics listen", "error", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be caught, so don't need to add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		return err
	}
	if s.Cfg.MetricsEnabled {
		if err := metricsSrv.Shutdown(ctx); err != nil {
			slog.Error("Metrics server forced to shutdown", "error", err)
			return err
		}
	}

	slog.Info("Server exiting")
	return nil
}

func (s *Server) buildRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		slog.Error("Panic recovered", "error", recovered, "path", c.Request.URL.Path, "method", c.Request.Method)
		c.AbortWithStatus(http.StatusInternalServerError)
	}))
	router.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"client_ip", c.ClientIP(),
		}

		switch {
		case status >= http.StatusInternalServerError:
			slog.Error("HTTP request", attrs...)
		case status >= http.StatusBadRequest:
			slog.Warn("HTTP request", attrs...)
		default:
			slog.Debug("HTTP request", attrs...)
		}
	})

	// Always configure trusted proxies explicitly.
	// Empty list disables proxy headers for ClientIP() and avoids spoofing via X-Forwarded-For.
	trustedProxies := s.Cfg.TrustedProxies
	if len(trustedProxies) == 0 {
		trustedProxies = nil
	}
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		slog.Warn("Invalid TRUSTED_PROXIES configuration", "error", err)
	}

	// CORS setup
	allowAnyOrigin := len(s.Cfg.AllowedOrigins) == 1 && s.Cfg.AllowedOrigins[0] == "*"
	router.Use(cors.New(cors.Config{
		AllowMethods: []string{"POST", "OPTIONS", "GET"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		// Do not allow credentials with wildcard origins.
		AllowCredentials: !allowAnyOrigin,
		AllowOriginFunc: func(origin string) bool {
			if allowAnyOrigin {
				return true
			}
			for _, allowedOrigin := range s.Cfg.AllowedOrigins {
				if allowedOrigin == origin {
					return true
				}
			}
			return false
		},
	}))

	// Limit request body size to prevent DoS via oversized payloads.
	router.Use(func(c *gin.Context) {
		if s.Cfg.MaxRequestBytes > 0 && c.Request.ContentLength > s.Cfg.MaxRequestBytes {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Request body too large"})
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, s.Cfg.MaxRequestBytes)
		c.Next()
	})
	router.Use(httpMetricsMiddleware())

	// Public API - not database-scoped, always at fixed path
	router.GET("/health", s.handleHealthCheck)

	prefix := s.Cfg.APIPrefix

	// Primary database endpoints under /{prefix}/:database/
	// JSON-RPC 2.0 gateway
	router.POST("/"+prefix+"/:database/jsonrpc", s.handleFunctionCall)
	router.POST("/"+prefix+"/:database/jsonrpc/", s.handleFunctionCall)
	// SSE stream for PostgreSQL NOTIFY
	router.GET("/"+prefix+"/:database/sse", s.handleSSE)

	// Backward-compatibility redirects from the legacy URL layout.
	// 307 Temporary Redirect is used intentionally: it preserves the HTTP method
	// (POST stays POST), which is required for JSON-RPC clients that do not
	// automatically update their endpoints.
	// These redirects remain registered regardless of the configured prefix so
	// that operators can safely migrate from the old "/api" and "/sse" paths to
	// the new layout without breaking existing clients.
	router.POST("/api/:database", legacyJSONRPCRedirect(prefix))
	router.POST("/api/:database/", legacyJSONRPCRedirect(prefix))
	router.GET("/sse/:database", legacySSERedirect(prefix))

	// Static files
	if s.Cfg.StaticFilesPath != "" {
		// Use NoRoute to serve static files when no other route matches.
		// Fallback to root 404.html if file not found (useful for SPA or clean documentation).
		router.NoRoute(func(c *gin.Context) {
			path := filepath.Join(s.Cfg.StaticFilesPath, filepath.Clean(c.Request.URL.Path))

			// 1. Try exact file
			if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
				c.File(path)
				return
			}

			// 2. Try index.html in directory
			indexPath := filepath.Join(path, "index.html")
			if fi, err := os.Stat(indexPath); err == nil && !fi.IsDir() {
				c.File(indexPath)
				return
			}

			// 3. Fallback to 404.html in the root
			errorPage := filepath.Join(s.Cfg.StaticFilesPath, "404.html")
			if fi, err := os.Stat(errorPage); err == nil && !fi.IsDir() {
				c.Status(http.StatusNotFound)
				c.File(errorPage)
				return
			}

			// 4. Final default
			c.String(http.StatusNotFound, "404 page not found")
		})
		slog.Info("Serving static files with 404 fallback", "path", s.Cfg.StaticFilesPath)
	}

	return router
}

func (s *Server) buildMetricsRouter() http.Handler {
	mux := http.NewServeMux()
	if s.Cfg.MetricsEnabled {
		mux.Handle("/metrics", promhttp.Handler())
	}
	return mux
}

// dbJSONRPCPath builds the canonical JSON-RPC path for a given database.
func (s *Server) dbJSONRPCPath(database string) string {
	return "/" + s.Cfg.APIPrefix + "/" + database + "/jsonrpc"
}

// dbSSEPath builds the canonical SSE path for a given database.
func (s *Server) dbSSEPath(database string) string {
	return "/" + s.Cfg.APIPrefix + "/" + database + "/sse"
}

// legacyJSONRPCRedirect returns a handler that issues a 307 redirect from the
// legacy POST /api/:database path to the current POST /{prefix}/:database/jsonrpc.
// 307 preserves the request method and body, which is critical for JSON-RPC POST
// clients that would otherwise lose their payload on a 301/302 redirect.
func legacyJSONRPCRedirect(prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		target := "/" + prefix + "/" + c.Param("database") + "/jsonrpc"
		if q := c.Request.URL.RawQuery; q != "" {
			target += "?" + q
		}
		c.Redirect(http.StatusTemporaryRedirect, target)
	}
}

// legacySSERedirect returns a handler that issues a 307 redirect from the
// legacy GET /sse/:database path to the current GET /{prefix}/:database/sse.
// Query parameters (e.g. "channels") are preserved transparently.
func legacySSERedirect(prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		target := "/" + prefix + "/" + c.Param("database") + "/sse"
		if q := c.Request.URL.RawQuery; q != "" {
			target += "?" + q
		}
		c.Redirect(http.StatusTemporaryRedirect, target)
	}
}

func (s *Server) authenticateToken(c *gin.Context, db *sql.DB, databaseName string) (string, string, int) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		recordAuthResult("unknown", "missing_header")
		return "", "Authorization header is missing", http.StatusUnauthorized
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		recordAuthResult("unknown", "malformed_header")
		return "", "Authorization header is malformed", http.StatusUnauthorized
	}

	authType := parts[0]
	tokenString := parts[1]

	// 1. Try JWT
	if strings.ToLower(authType) == "bearer" {
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(s.Cfg.JWTSecret), nil
		})

		if err == nil && token.Valid {
			claims, ok := token.Claims.(jwt.MapClaims)
			dbRole, roleOk := claims["db_role"].(string)
			dbName, dbNameOk := claims["db_name"].(string)

			if ok && roleOk && dbRole != "" && dbNameOk {
				if dbName != databaseName {
					slog.Warn("JWT token used for wrong database", "token_db", dbName, "requested_db", databaseName)
					recordAuthResult("jwt", "wrong_db")
					return "", "Invalid token for this database", http.StatusUnauthorized
				}
				recordAuthResult("jwt", "success")
				return dbRole, "", 0
			}
		}
		recordAuthResult("jwt", "invalid")
	}

	// 2. Try Long-lived API Token
	query := `SELECT current_catalog, pgarachne.verify_api_token($1)`

	var (
		currentCatalog string
		nullRole       sql.NullString
	)
	err := db.QueryRowContext(c.Request.Context(), query, tokenString).Scan(&currentCatalog, &nullRole)

	if err == nil && nullRole.Valid {
		if currentCatalog != databaseName {
			slog.Warn("API token used for wrong database", "token_db", currentCatalog, "requested_db", databaseName)
			recordAuthResult("api_token", "wrong_db")
			return "", "Invalid token for this database", http.StatusUnauthorized
		}

		recordAuthResult("api_token", "success")
		return nullRole.String, "", 0
	}

	recordAuthResult("api_token", "invalid")
	return "", "Invalid or expired token", http.StatusUnauthorized
}

func (s *Server) handleFunctionCall(c *gin.Context) {
	databaseName := c.Param("database")

	if !isSafeDatabaseName(databaseName) {
		c.JSON(http.StatusBadRequest, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid database name"}})
		return
	}

	var req JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid JSON request"}})
		return
	}

	functionName := strings.TrimSpace(req.Method)
	if functionName == "" {
		recordJSONRPC("", "error")
		c.JSON(http.StatusBadRequest, JSONRPCResponse{Error: &JSONRPCError{Message: "JSON-RPC method is required"}, ID: req.ID})
		return
	}

	if !isSafeFunctionName(functionName) {
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusBadRequest, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid function name"}, ID: req.ID})
		return
	}

	c.Set("jsonrpc_id", req.ID)

	if functionName == "login" {
		s.handleLoginRPC(c, req, databaseName)
		return
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		recordAuthResult("unknown", "missing_header")
		c.JSON(http.StatusUnauthorized, JSONRPCResponse{Error: &JSONRPCError{Message: "Authorization header is missing"}, ID: req.ID})
		return
	}
	if parts := strings.SplitN(authHeader, " ", 2); len(parts) != 2 {
		recordAuthResult("unknown", "malformed_header")
		c.JSON(http.StatusUnauthorized, JSONRPCResponse{Error: &JSONRPCError{Message: "Authorization header is malformed"}, ID: req.ID})
		return
	}

	db, err := database.GetConnection(s.Cfg, databaseName)
	if err != nil {
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusServiceUnavailable, JSONRPCResponse{Error: &JSONRPCError{Message: "Database connection failed"}, ID: req.ID})
		return
	}

	dbRole, errMsg, status := s.authenticateToken(c, db, databaseName)
	if errMsg != "" {
		c.JSON(status, JSONRPCResponse{Error: &JSONRPCError{Message: errMsg}, ID: req.ID})
		return
	}

	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Message: "Failed to marshal params"}, ID: req.ID})
		return
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		slog.Error("Failed to begin transaction", "error", err)
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusServiceUnavailable, JSONRPCResponse{Error: &JSONRPCError{Message: "Database unavailable"}, ID: req.ID})
		return
	}
	defer tx.Rollback()

	// Idempotency check — runs as the service user (DB_USER) before SET LOCAL ROLE,
	// so no extra privileges are needed on the client role side.
	// The check is intentionally inside the transaction: if the function call fails
	// and the transaction rolls back, the key is not persisted and the client may retry.
	if req.IdempotencyKey != "" {
		var saved bool
		err := tx.QueryRowContext(
			c.Request.Context(),
			`SELECT pgarachne.save_idempotency_key($1)`,
			req.IdempotencyKey,
		).Scan(&saved)
		if err != nil {
			slog.Error("Idempotency check failed", "key", req.IdempotencyKey, "function", functionName, "error", err)
			recordJSONRPC(functionName, "error")
			c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Message: "Idempotency check failed"}, ID: req.ID})
			return
		}
		if !saved {
			slog.Warn("Duplicate request rejected", "key", req.IdempotencyKey, "function", functionName)
			recordJSONRPC(functionName, "duplicate")
			c.JSON(http.StatusConflict, JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32000, Message: "This request has already been processed"},
				ID:      req.ID,
			})
			return
		}
	}

	// Safe identifier quoting for role
	quotedRole := fmt.Sprintf(`"%s"`, strings.ReplaceAll(dbRole, `"`, `""`))
	if _, err := tx.ExecContext(c.Request.Context(), fmt.Sprintf("SET LOCAL ROLE %s", quotedRole)); err != nil {
		slog.Error("Failed to SET ROLE", "role", dbRole, "error", err)
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusForbidden, JSONRPCResponse{Error: &JSONRPCError{Code: -32001, Message: "Permission denied for the specified role"}, ID: req.ID})
		return
	}

	// Call the function
	var query string
	if functionName == "capabilities" || functionName == "pgarachne.capabilities" {
		query = `SELECT pgarachne.capabilities($1::jsonb)::json`
	} else {
		// Allow schema-qualified function names (e.g., api.server_info)
		query = fmt.Sprintf("SELECT %s($1::jsonb)::json", functionName)
	}

	var resultJSON json.RawMessage
	err = tx.QueryRowContext(c.Request.Context(), query, paramsJSON).Scan(&resultJSON)
	if err != nil {
		slog.Error("Function call failed", "function", functionName, "error", err)
		recordJSONRPC(functionName, "error")
		if strings.Contains(err.Error(), "does not exist") {
			c.JSON(http.StatusNotFound, JSONRPCResponse{Error: &JSONRPCError{Message: "Function does not exist"}, ID: req.ID})
		} else {
			c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Message: "Function call failed"}, ID: req.ID})
		}
		return
	}

	if err := tx.Commit(); err != nil {
		slog.Error("Transaction commit failed", "error", err)
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Message: "Transaction commit failed"}, ID: req.ID})
		return
	}

	recordJSONRPC(functionName, "success")
	c.JSON(http.StatusOK, JSONRPCResponse{
		JSONRPC: "2.0", Result: resultJSON, ID: req.ID,
	})
}

func (s *Server) handleLoginRPC(c *gin.Context, req JSONRPCRequest, databaseName string) {
	var loginReq LoginRequest
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		c.JSON(http.StatusBadRequest, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid params"}, ID: req.ID})
		return
	}
	if err := json.Unmarshal(paramsJSON, &loginReq); err != nil {
		c.JSON(http.StatusBadRequest, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid params"}, ID: req.ID})
		return
	}

	if loginAttemptLimiter != nil && !loginAttemptLimiter.Allow(c.ClientIP()+"|"+loginReq.Login) {
		recordLoginResult("rate_limited")
		c.JSON(http.StatusTooManyRequests, JSONRPCResponse{Error: &JSONRPCError{Message: "Too many login attempts. Please try again later."}, ID: req.ID})
		return
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s %s",
		s.Cfg.DBHost,
		s.Cfg.DBPort,
		config.QuoteConninfoValue(loginReq.Login),
		config.QuoteConninfoValue(loginReq.Password),
		config.QuoteConninfoValue(databaseName),
		s.Cfg.DBSSLParams(),
	)

	tempDB, err := sql.Open("postgres", connStr)
	if err != nil {
		slog.Error("Failed to open verification connection", "error", err)
		recordLoginResult("error")
		c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Message: "Internal authentication error"}, ID: req.ID})
		return
	}
	defer tempDB.Close()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if err := tempDB.PingContext(ctx); err != nil {
		slog.Warn("Authentication failed", "user", loginReq.Login, "error", err)
		recordLoginResult("invalid")
		c.JSON(http.StatusUnauthorized, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid login or password"}, ID: req.ID})
		return
	}

	expirationTime := time.Now().Add(time.Duration(s.Cfg.JWTExpiryHours) * time.Hour)
	claims := jwt.MapClaims{"db_role": loginReq.Login, "db_name": databaseName, "exp": expirationTime.Unix()}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.Cfg.JWTSecret))
	if err != nil {
		slog.Error("Failed to sign JWT", "error", err)
		recordLoginResult("error")
		c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Message: "Failed to create session token"}, ID: req.ID})
		return
	}

	resultBytes, err := json.Marshal(map[string]string{"token": tokenString})
	if err != nil {
		slog.Error("Failed to marshal login response", "error", err)
		recordLoginResult("error")
		c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Message: "Failed to create session token"}, ID: req.ID})
		return
	}

	recordLoginResult("success")
	c.JSON(http.StatusOK, JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  resultBytes,
		ID:      req.ID,
	})
}

func (s *Server) handleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func isSafeDatabaseName(name string) bool {
	if name == "" {
		return false
	}
	return pgIdentRe.MatchString(name) || pgQuotedIdentRe.MatchString(name)
}

func isSafeFunctionName(name string) bool {
	if name == "" {
		return false
	}
	if name == "capabilities" || name == "login" {
		return true
	}
	return pgFunctionRe.MatchString(name)
}

func initLoginLimiter(cfg *config.Config) {
	if cfg == nil {
		loginAttemptLimiter = newLoginLimiter(5, time.Minute)
		return
	}
	if cfg.LoginRateLimit == 0 {
		loginAttemptLimiter = nil
		return
	}
	loginAttemptLimiter = newLoginLimiter(cfg.LoginRateLimit, cfg.LoginRateWindow)
}

type loginLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	entries     map[string][]time.Time
	lastCleanup time.Time
}

func newLoginLimiter(limit int, window time.Duration) *loginLimiter {
	return &loginLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string][]time.Time),
	}
}

func (l *loginLimiter) Allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.lastCleanup.IsZero() || now.Sub(l.lastCleanup) > l.window {
		for k, entries := range l.entries {
			kept := entries[:0]
			for _, t := range entries {
				if t.After(cutoff) {
					kept = append(kept, t)
				}
			}
			if len(kept) == 0 {
				delete(l.entries, k)
				continue
			}
			l.entries[k] = kept
		}
		l.lastCleanup = now
	}

	entries := l.entries[key]
	kept := entries[:0]
	for _, t := range entries {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= l.limit {
		l.entries[key] = kept
		return false
	}

	kept = append(kept, now)
	l.entries[key] = kept
	return true
}
