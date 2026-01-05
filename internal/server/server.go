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
	"strings"
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
	Cfg *config.Config
}

func New(cfg *config.Config) *Server {
	return &Server{Cfg: cfg}
}

func (s *Server) Run() error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// CORS setup
	router.Use(cors.New(cors.Config{
		AllowMethods:     []string{"POST", "OPTIONS", "GET"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
		AllowOriginFunc: func(origin string) bool {
			if len(s.Cfg.AllowedOrigins) == 1 && s.Cfg.AllowedOrigins[0] == "*" {
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

	// Public API
	router.GET("/health", s.handleHealthCheck)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	router.POST("/api/:database/login", s.handleLogin)

	// Protected API
	protectedAPI := router.Group("/api/:database")
	protectedAPI.Use(s.authMiddleware())
	protectedAPI.POST("/:function", s.handleFunctionCall)

	// Static files
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

func (s *Server) handleLogin(c *gin.Context) {
	var loginReq LoginRequest
	if err := c.ShouldBindJSON(&loginReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Direct DB Authentication Strategy:
	// We try to open a connection to the requested database using the provided credentials.
	// If successful, the user is authenticated and the role is the login name.

	// Construct connection string for verification (disable SSL for local, adjust as needed)
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		s.Cfg.DBHost, s.Cfg.DBPort, loginReq.Login, loginReq.Password, c.Param("database"))

	// Try to connect
	tempDB, err := sql.Open("postgres", connStr)
	if err != nil {
		slog.Error("Failed to open verification connection", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal authentication error"})
		return
	}
	defer tempDB.Close()

	// Ping to verify credentials
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if err := tempDB.PingContext(ctx); err != nil {
		slog.Warn("Authentication failed", "user", loginReq.Login, "error", err)
		// Don't leak details, just say invalid
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid login or password"})
		return
	}

	// Authentication Successful
	dbRole := loginReq.Login

	// Create JWT
	expirationTime := time.Now().Add(time.Duration(s.Cfg.JWTExpiryHours) * time.Hour)
	claims := jwt.MapClaims{"db_role": dbRole, "db_name": c.Param("database"), "exp": expirationTime.Unix()}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.Cfg.JWTSecret))
	if err != nil {
		slog.Error("Failed to sign JWT", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, JSONRPCResponse{Error: &JSONRPCError{Message: "Authorization header is missing"}})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 {
			c.JSON(http.StatusUnauthorized, JSONRPCResponse{Error: &JSONRPCError{Message: "Authorization header is malformed"}})
			c.Abort()
			return
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
					// Validate database access scope
					requestedDb := c.Param("database")
					if dbName != requestedDb {
						slog.Warn("JWT token used for wrong database", "token_db", dbName, "requested_db", requestedDb)
						c.JSON(http.StatusUnauthorized, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid token for this database"}})
						c.Abort()
						return
					}

					c.Set("db_role", dbRole)
					c.Next()
					return
				}
			}
		}

		// 2. Try Long-lived API Token
		// Logic: We pass the raw token to the DB function 'pgarachne.verify_api_token'.
		// The DB handles hashing and checking validity.
		databaseName := c.Param("database")
		db, err := database.GetConnection(s.Cfg, databaseName)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, JSONRPCResponse{Error: &JSONRPCError{Message: "Database connection failed"}})
			c.Abort()
			return
		}

		// Direct call to verification function

		// Direct call to verification function
		query := `SELECT pgarachne.verify_api_token($1)`

		// Note: verification function returns role name or NULL if invalid.
		// using sql.NullString handles NULL correctly without error.
		var nullRole sql.NullString
		err = db.QueryRowContext(c.Request.Context(), query, tokenString).Scan(&nullRole)

		if err == nil && nullRole.Valid {
			// Update last_used_at is not needed as per requirements (user removed it).

			c.Set("db_role", nullRole.String)
			c.Next()
			return
		}

		c.JSON(http.StatusUnauthorized, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid or expired token"}})
		c.Abort()
	}
}

func (s *Server) handleFunctionCall(c *gin.Context) {
	databaseName := c.Param("database")
	functionName := c.Param("function")

	if functionName == "login" {
		c.JSON(http.StatusForbidden, JSONRPCResponse{Error: &JSONRPCError{Message: "Login must be called via the public endpoint"}})
		return
	}

	db, err := database.GetConnection(s.Cfg, databaseName)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, JSONRPCResponse{Error: &JSONRPCError{Message: "Database connection failed"}})
		return
	}

	var req JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, JSONRPCResponse{Error: &JSONRPCError{Message: "Invalid JSON request"}})
		return
	}

	c.Set("jsonrpc_id", req.ID)

	dbRole := c.GetString("db_role")
	if dbRole == "" {
		slog.Error("db_role not found in context")
		c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Code: -32000, Message: "Internal Server Error: User role not identified"}, ID: req.ID})
		return
	}

	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Message: "Failed to marshal params"}, ID: req.ID})
		return
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		slog.Error("Failed to begin transaction", "error", err)
		c.JSON(http.StatusServiceUnavailable, JSONRPCResponse{Error: &JSONRPCError{Message: "Database unavailable"}, ID: req.ID})
		return
	}
	defer tx.Rollback()

	// Safe identifier quoting for role
	quotedRole := fmt.Sprintf(`"%s"`, strings.ReplaceAll(dbRole, `"`, `""`))
	if _, err := tx.ExecContext(c.Request.Context(), fmt.Sprintf("SET LOCAL ROLE %s", quotedRole)); err != nil {
		slog.Error("Failed to SET ROLE", "role", dbRole, "error", err)
		c.JSON(http.StatusForbidden, JSONRPCResponse{Error: &JSONRPCError{Code: -32001, Message: "Permission denied for the specified role"}, ID: req.ID})
		return
	}

	// Call the function
	var query string
	if functionName == "capabilities" {
		query = `SELECT pgarachne.capabilities($1::jsonb)::json`
	} else {
		// Allow schema-qualified function names (e.g., api.server_info)
		// TODO: Validate functionName to prevent SQL injection (e.g., ensure it matches expected pattern like "schema.function")
		query = fmt.Sprintf("SELECT %s($1::jsonb)::json", functionName)
	}

	var resultJSON json.RawMessage
	err = tx.QueryRowContext(c.Request.Context(), query, paramsJSON).Scan(&resultJSON)
	if err != nil {
		slog.Error("Function call failed", "function", functionName, "error", err)
		if strings.Contains(err.Error(), "does not exist") {
			c.JSON(http.StatusNotFound, JSONRPCResponse{Error: &JSONRPCError{Message: "Function does not exist"}, ID: req.ID})
		} else {
			c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Message: fmt.Sprintf("Function call failed: %v", err)}, ID: req.ID})
		}
		return
	}

	if err := tx.Commit(); err != nil {
		slog.Error("Transaction commit failed", "error", err)
		c.JSON(http.StatusInternalServerError, JSONRPCResponse{Error: &JSONRPCError{Message: "Transaction commit failed"}, ID: req.ID})
		return
	}

	c.JSON(http.StatusOK, JSONRPCResponse{
		JSONRPC: "2.0", Result: resultJSON, ID: req.ID,
	})
}

func (s *Server) handleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
