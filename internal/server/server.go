package server

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/heptau/pgarachne/internal/auth"
	"github.com/heptau/pgarachne/internal/config"
	"github.com/heptau/pgarachne/internal/database"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	Cfg     *config.Config
	sseHub  *sseHub
	jwtSign *auth.Signer
	// loginLimiter throttles attempts per (client IP, username) pair;
	// ipLoginLimiter throttles per client IP regardless of username, so an
	// attacker cannot get a fresh budget by rotating usernames.
	loginLimiter   *loginLimiter
	ipLoginLimiter *loginLimiter
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
)

func New(cfg *config.Config) *Server {
	return &Server{
		Cfg:            cfg,
		sseHub:         newSSEHub(cfg),
		jwtSign:        auth.NewSigner(&auth.Options{Issuer: cfg.JWTIssuer, Audience: cfg.JWTAudience, Leeway: cfg.JWTLeeway}),
		loginLimiter:   newLoginLimiterFromConfig(cfg),
		ipLoginLimiter: newIPLoginLimiterFromConfig(cfg),
	}
}

func (s *Server) Run() error {
	gin.SetMode(gin.ReleaseMode)
	router := s.buildRouter()
	metricsRouter := s.buildMetricsRouter()

	srv := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second, // generous for SSE; ResponseController overrides per-stream
		IdleTimeout:       2 * time.Minute,
	}
	metricsSrv := &http.Server{
		Handler:           metricsRouter,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       2 * time.Minute,
		WriteTimeout:      15 * time.Second,
	}

	// Bind the port before starting the goroutine so that a "port already in
	// use" error is returned synchronously from Run() rather than being lost
	// inside a goroutine where it can only be logged.
	ln, err := net.Listen("tcp", ":"+s.Cfg.HTTPPort)
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", s.Cfg.HTTPPort, err)
	}
	slog.Info("Starting PgArachne server", "port", s.Cfg.HTTPPort)
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	if s.Cfg.MetricsEnabled {
		metricsLn, err := net.Listen("tcp", s.Cfg.MetricsListenAddr)
		if err != nil {
			shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutCancel()
			_ = srv.Shutdown(shutCtx)
			return fmt.Errorf("failed to listen on metrics addr %s: %w", s.Cfg.MetricsListenAddr, err)
		}
		slog.Info("Starting metrics server", "listen_addr", s.Cfg.MetricsListenAddr)
		go func() {
			if err := metricsSrv.Serve(metricsLn); err != nil && err != http.ErrServerClosed {
				slog.Error("metrics server error", "error", err)
			}
		}()
	} else {
		slog.Info("Metrics endpoint disabled")
	}

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

	// SSE hub and DB pools get their own deadline. The HTTP server is
	// already drained, so SSE/DB shutdown only has to release sockets
	// and goroutines — it should be fast (sub-second in practice). We
	// keep a generous timeout in case a slow Postgres is involved.
	sseCtx, sseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sseCancel()
	if err := s.sseHub.Shutdown(sseCtx); err != nil {
		slog.Warn("SSE hub shutdown incomplete", "error", err)
	}
	// DB pools are independent of the HTTP lifecycle and must be closed
	// explicitly. Without this, sql.DB holds idle connections open until
	// the OS reaps them or PostgreSQL's idle_in_transaction_session_timeout
	// fires, both of which are out of our control.
	database.CloseAll()

	slog.Info("Server exiting")
	return nil
}

func (s *Server) buildRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		slog.Error("Panic recovered", "error", recovered, "path", c.Request.URL.Path, "method", c.Request.Method, "stack", string(buf[:n]))
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

	// MCP (Model Context Protocol) endpoint — Streamable HTTP transport.
	// Translates MCP tools/list and tools/call to PostgreSQL function calls,
	// using the same auth and role-switching logic as the JSON-RPC endpoint.
	router.POST("/"+prefix+"/:database/mcp", s.handleMCP)

	// OpenAPI 3.1 spec, generated by pgarachne.generate_openapi_spec on
	// the fly. The spec is database-scoped because each database exposes
	// its own set of JSON-RPC methods. Returns the spec as
	// application/json; OpenAPI tooling can consume the response directly.
	router.GET("/"+prefix+"/:database/openapi.json", s.handleOpenAPISpec)

	// Static files
	if s.Cfg.StaticFilesPath != "" {
		staticRoot, err := filepath.Abs(s.Cfg.StaticFilesPath)
		if err != nil {
			slog.Error("Failed to resolve static files path", "path", s.Cfg.StaticFilesPath, "error", err)
			staticRoot = s.Cfg.StaticFilesPath
		}

		// Use NoRoute to serve static files when no other route matches.
		// Fallback to root 404.html if file not found (useful for SPA or clean documentation).
		router.NoRoute(func(c *gin.Context) {
			path := filepath.Join(staticRoot, filepath.Clean(c.Request.URL.Path))

			// Belt-and-suspenders: filepath.Clean on an absolute URL path
			// already collapses any ".." above the root, but verify
			// containment explicitly so this can never serve a file outside
			// staticRoot regardless of how path was built above.
			if !isWithinDir(path, staticRoot) {
				c.String(http.StatusNotFound, "404 page not found")
				return
			}

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
			errorPage := filepath.Join(staticRoot, "404.html")
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
		claims, err := s.jwtSign.Parse(s.Cfg.JWTSecret, tokenString)
		if err == nil {
			if claims.DBName != databaseName {
				slog.Warn("JWT token used for wrong database", "token_db", claims.DBName, "requested_db", databaseName)
				recordAuthResult("jwt", "wrong_db")
				return "", "Invalid token for this database", http.StatusUnauthorized
			}
			recordAuthResult("jwt", "success")
			return claims.DBRole, "", 0
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
	if len(functionName) > MaxMethodLength {
		recordJSONRPC("", "error")
		c.JSON(http.StatusBadRequest, JSONRPCResponse{Error: &JSONRPCError{Message: "JSON-RPC method is too long"}, ID: req.ID})
		return
	}
	if len(req.IdempotencyKey) > MaxIdempotencyKeyLength {
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusBadRequest, JSONRPCResponse{Error: &JSONRPCError{Message: "idempotencyKey is too long"}, ID: req.ID})
		return
	}

	if !isSafeFunctionName(functionName) {
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusBadRequest, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid function name"}, ID: req.ID})
		return
	}

	c.Set("jsonrpc_id", req.ID)

	if functionName == "get_jwt" {
		s.handleLoginRPC(c, req, databaseName)
		return
	}

	// --- Authentication ---
	// Three modes, checked in order:
	//   1. Basic Auth   → direct DB connection as the user; no SET LOCAL ROLE.
	//   2. Bearer JWT   → system DB connection; SET LOCAL ROLE to the JWT subject.
	//   3. API token    → system DB connection; SET LOCAL ROLE to the token's role.
	//
	// For modes 2 and 3, dbRole is non-empty and SET LOCAL ROLE is performed.
	// For mode 1, dbRole is "" and SET LOCAL ROLE is skipped — the connection is
	// already authenticated as the user.

	var execDB *sql.DB
	var dbRole string // "" signals direct auth — skip SET LOCAL ROLE below

	if username, password, ok := parseBasicAuth(c.GetHeader("Authorization")); ok {
		// Direct credential auth: validate size then open/reuse a user pool.
		if len(username) == 0 || len(username) > MaxLoginLength || len(password) > MaxPasswordLength {
			recordAuthResult("direct", "malformed")
			c.JSON(http.StatusUnauthorized, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid credentials"}, ID: req.ID})
			return
		}
		userDB, err := database.GetUserConnection(s.Cfg, databaseName, username, password)
		if err != nil {
			slog.Warn("Direct authentication failed", "user", username, "database", databaseName, "error", err)
			recordAuthResult("direct", "invalid")
			c.JSON(http.StatusUnauthorized, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid credentials"}, ID: req.ID})
			return
		}
		recordAuthResult("direct", "success")
		execDB = userDB
		// dbRole stays "" — SET LOCAL ROLE is skipped in the execution block below.
	} else {
		// JWT / API token auth (existing behaviour).
		sysDB, err := database.GetConnection(s.Cfg, databaseName)
		if err != nil {
			recordJSONRPC(functionName, "error")
			c.JSON(http.StatusServiceUnavailable, JSONRPCResponse{Error: &JSONRPCError{Message: "Database connection failed"}, ID: req.ID})
			return
		}
		role, errMsg, status := s.authenticateToken(c, sysDB, databaseName)
		if errMsg != "" {
			c.JSON(status, JSONRPCResponse{Error: &JSONRPCError{Message: errMsg}, ID: req.ID})
			return
		}
		execDB = sysDB
		dbRole = role
	}

	paramsJSON := req.Params
	if len(paramsJSON) == 0 || string(paramsJSON) == "null" {
		paramsJSON = json.RawMessage("{}")
	}

	tx, err := execDB.BeginTx(c.Request.Context(), nil)
	if err != nil {
		slog.Error("Failed to begin transaction", "error", err)
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusServiceUnavailable, JSONRPCResponse{Error: &JSONRPCError{Message: "Database unavailable"}, ID: req.ID})
		return
	}
	defer rollbackQuietly(tx)

	if err := s.setupRequestTx(c.Request.Context(), tx, dbRole, req.IdempotencyKey); err != nil {
		switch {
		case errors.Is(err, errIdempotencyDuplicate):
			slog.Warn("Duplicate request rejected", "key", req.IdempotencyKey, "function", functionName)
			recordJSONRPC(functionName, "duplicate")
			c.JSON(http.StatusConflict, JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32000, Message: "This request has already been processed"},
				ID:      req.ID,
			})
		case errors.Is(err, errIdempotencyCheckFailed):
			slog.Error("Idempotency check failed", "key", req.IdempotencyKey, "function", functionName, "error", err)
			recordJSONRPC(functionName, "error")
			c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Message: "Idempotency check failed"}, ID: req.ID})
		default:
			slog.Error("Failed to SET ROLE", "role", dbRole, "error", err)
			recordJSONRPC(functionName, "error")
			c.JSON(http.StatusForbidden, JSONRPCResponse{Error: &JSONRPCError{Code: -32001, Message: "Permission denied for the specified role"}, ID: req.ID})
		}
		return
	}

	// functionName is part of SQL syntax (it names the function to call), not
	// a value, so it can't be passed as a bind parameter — isSafeFunctionName
	// above is the mitigation instead, requiring a strict schema.function
	// identifier shape before functionName ever reaches buildFunctionQuery.
	query := buildFunctionQuery(functionName)

	var resultJSON json.RawMessage
	err = tx.QueryRowContext(c.Request.Context(), query, paramsJSON).Scan(&resultJSON) // codeql[go/sql-injection] -- functionName is validated by isSafeFunctionName() before reaching here
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
	params := req.Params
	if len(params) == 0 || string(params) == "null" {
		params = json.RawMessage("{}")
	}
	if err := json.Unmarshal(params, &loginReq); err != nil {
		c.JSON(http.StatusBadRequest, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid params"}, ID: req.ID})
		return
	}
	// Reject oversized credentials before they reach the rate limiter or
	// the connection string. The limits (types.go) are wide enough for any
	// legitimate user but keep an attacker from blowing up memory with a
	// single multi-megabyte "password" field.
	if len(loginReq.Login) > MaxLoginLength {
		recordLoginResult("invalid")
		c.JSON(http.StatusBadRequest, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid login or password"}, ID: req.ID})
		return
	}
	if len(loginReq.Password) > MaxPasswordLength {
		recordLoginResult("invalid")
		c.JSON(http.StatusBadRequest, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid login or password"}, ID: req.ID})
		return
	}

	// Per-IP check first: it has the higher limit, and checking it before the
	// per-(IP, username) limiter means a spraying attacker burns the IP budget
	// even when each individual username is below its own limit.
	if s.ipLoginLimiter != nil && !s.ipLoginLimiter.Allow(c.ClientIP()) {
		recordLoginResult("rate_limited")
		c.JSON(http.StatusTooManyRequests, JSONRPCResponse{Error: &JSONRPCError{Message: "Too many login attempts. Please try again later."}, ID: req.ID})
		return
	}
	if s.loginLimiter != nil && !s.loginLimiter.Allow(c.ClientIP()+"|"+loginReq.Login) {
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

	tokenString, err := s.jwtSign.Issue(s.Cfg.JWTSecret, loginReq.Login, databaseName, time.Duration(s.Cfg.JWTExpiryHours)*time.Hour)
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
		c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Message: "Failed to encode session response"}, ID: req.ID})
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

// handleOpenAPISpec returns an OpenAPI 3.1 document for the requested
// database. The spec is generated on the server by calling
// pgarachne.generate_openapi_spec with the request's Host header as the
// public base URL (so the spec is portable across reverse-proxy
// configurations). The endpoint is intentionally unauthenticated — the
// spec only describes method *names* and *descriptions*, not data — and
// it is served as application/json, which is what OpenAPI tooling
// (Swagger UI, Redoc, openapi-generator) expects.
//
// We do not cache the result because pgarachne.capabilities() reflects
// the live state of pg_proc and any function changes (additions, drops,
// comment edits) should be visible in the spec immediately. The
// pgarachne service user executes generate_openapi_spec (it is granted
// to public), so the route is reachable without an authenticated role.
func (s *Server) handleOpenAPISpec(c *gin.Context) {
	databaseName := c.Param("database")
	if !isSafeDatabaseName(databaseName) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid database name"})
		return
	}

	db, err := database.GetConnection(s.Cfg, databaseName)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database connection failed"})
		return
	}

	// Build a best-effort public base URL. The Host header is what the
	// client sent; in a typical deployment behind a reverse proxy this
	// will be the public hostname. Operators who need a different
	// origin in the spec can override the apiPrefix config and put
	// their public host in front.
	//
	// X-Forwarded-Proto is only trusted when the request originates from a
	// configured trusted proxy (gin sets c.Request.Header only after proxy
	// validation). We check TLS first; only fall back to the header when
	// TrustedProxies is non-empty, mirroring gin's own proxy-trust logic.
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	} else if len(s.Cfg.TrustedProxies) > 0 && strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	serverURLBase := scheme + "://" + c.Request.Host

	row := db.QueryRowContext(c.Request.Context(),
		`SELECT pgarachne.generate_openapi_spec($1)::text`, serverURLBase)
	var spec string
	if err := row.Scan(&spec); err != nil {
		slog.Warn("OpenAPI spec generation failed", "database", databaseName, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OpenAPI spec"})
		return
	}

	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, spec)
}

func isSafeDatabaseName(name string) bool {
	if name == "" {
		return false
	}
	return pgIdentRe.MatchString(name) || pgQuotedIdentRe.MatchString(name)
}

// quoteRole returns a double-quoted PostgreSQL identifier safe for use in
// SET LOCAL ROLE. Inner double-quotes are escaped by doubling them, which
// is the standard SQL identifier quoting rule (ISO/IEC 9075).
// buildFunctionQuery returns the SQL query string for calling a JSON-RPC
// function. The capabilities method is always routed to pgarachne.capabilities
// regardless of how the caller spelled it (with or without schema prefix).
func buildFunctionQuery(functionName string) string {
	if functionName == "capabilities" || functionName == "pgarachne.capabilities" {
		return `SELECT pgarachne.capabilities($1::jsonb)::json`
	}
	return fmt.Sprintf("SELECT %s($1::jsonb)::json", functionName)
}

// isWithinDir reports whether path is dir itself or a descendant of it.
// Both must already be absolute, cleaned paths.
func isWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func quoteRole(role string) string {
	return `"` + strings.ReplaceAll(role, `"`, `""`) + `"`
}

func isSafeFunctionName(name string) bool {
	if name == "" {
		return false
	}
	if name == "capabilities" || name == "get_jwt" {
		return true
	}
	return pgFunctionRe.MatchString(name)
}

// parseBasicAuth parses an HTTP Basic-Auth header value of the form
// "Basic base64(username:password)" and returns the decoded credentials.
// Returns ok=false for any malformed input (wrong scheme, bad base64, no colon).
// The password may itself contain colons — only the first colon is treated as
// the separator, consistent with RFC 7617 §2.
func parseBasicAuth(header string) (username, password string, ok bool) {
	const prefix = "Basic "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", "", false
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header[len(prefix):]))
	if err != nil {
		return "", "", false
	}
	pair := string(payload)
	idx := strings.IndexByte(pair, ':')
	if idx < 0 {
		return "", "", false
	}
	return pair[:idx], pair[idx+1:], true
}

func newLoginLimiterFromConfig(cfg *config.Config) *loginLimiter {
	if cfg == nil {
		return newLoginLimiter(5, time.Minute)
	}
	if cfg.LoginRateLimit == 0 {
		return nil
	}
	return newLoginLimiter(cfg.LoginRateLimit, cfg.LoginRateWindow)
}

func newIPLoginLimiterFromConfig(cfg *config.Config) *loginLimiter {
	if cfg == nil {
		return newLoginLimiter(25, time.Minute)
	}
	if cfg.LoginRateLimitPerIP == 0 {
		return nil
	}
	return newLoginLimiter(cfg.LoginRateLimitPerIP, cfg.LoginRateWindow)
}

type loginLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	// maxEntries caps the number of distinct keys tracked. Cleanup is lazy
	// (O(n) once per window), so under a high-cardinality attack (many unique
	// IPs) the map can grow unboundedly without this bound. When full, new
	// keys are rate-limited immediately (fail-closed) rather than admitted.
	maxEntries  int
	entries     map[string][]time.Time
	lastCleanup time.Time
}

func newLoginLimiter(limit int, window time.Duration) *loginLimiter {
	return &loginLimiter{
		limit:      limit,
		window:     window,
		maxEntries: 100_000,
		entries:    make(map[string][]time.Time),
	}
}

func (l *loginLimiter) Allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	// Trigger O(n) cleanup in a goroutine so the hot Allow path is never
	// blocked by full-map iteration. We snapshot lastCleanup under the lock,
	// launch the goroutine, and update lastCleanup before releasing — this
	// prevents multiple concurrent cleanups.
	if l.lastCleanup.IsZero() || now.Sub(l.lastCleanup) > l.window {
		l.lastCleanup = now
		go l.cleanup(cutoff)
	}

	var kept []time.Time
	for _, t := range l.entries[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= l.limit {
		l.entries[key] = kept
		return false
	}

	// New key but map is full — fail closed to prevent unbounded memory growth.
	if _, exists := l.entries[key]; !exists && l.maxEntries > 0 && len(l.entries) >= l.maxEntries {
		return false
	}

	kept = append(kept, now)
	l.entries[key] = kept
	return true
}

// cleanup removes expired entries from the map. Called from a goroutine to
// avoid blocking Allow() callers during full-map iteration.
func (l *loginLimiter) cleanup(cutoff time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, entries := range l.entries {
		var kept []time.Time
		for _, t := range entries {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(l.entries, k)
		} else {
			l.entries[k] = kept
		}
	}
}
