package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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

	slog.Info("Starting PgArachne server", "port", s.Cfg.HTTPPort)

	srv := &http.Server{
		Addr:    ":" + s.Cfg.HTTPPort,
		Handler: router,
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

	slog.Info("Server exiting")
	return nil
}

func (s *Server) buildRouter() *gin.Engine {
	router := gin.Default()
	if len(s.Cfg.TrustedProxies) > 0 {
		if err := router.SetTrustedProxies(s.Cfg.TrustedProxies); err != nil {
			slog.Warn("Invalid TRUSTED_PROXIES configuration", "error", err)
		}
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

	// Public API
	router.GET("/health", s.handleHealthCheck)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/sse/:database", s.handleSSE)

	// JSON-RPC API (all methods are invoked via /api/:database)
	router.POST("/api/:database", s.handleFunctionCall)
	router.POST("/api/:database/", s.handleFunctionCall)

	// Static files
	if s.Cfg.StaticFilesPath != "" {
		// Use NoRoute to serve static files when no other route matches.
		// This avoids conflicts with specific routes like /health at the root level.
		router.NoRoute(func(c *gin.Context) {
			fileServer := http.FileServer(http.Dir(s.Cfg.StaticFilesPath))
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
		slog.Info("Serving static files via fallback", "path", s.Cfg.StaticFilesPath)
	}

	return router
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
			c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Message: fmt.Sprintf("Function call failed: %v", err)}, ID: req.ID})
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
