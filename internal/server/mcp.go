package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/pgarachne/internal/database"
)

// mcpProtocolVersion is the MCP protocol version this server advertises.
// 2024-11-05 is the most widely supported version across clients.
const mcpProtocolVersion = "2024-11-05"

// JSON-RPC 2.0 error codes mandated by the MCP specification.
const (
	mcpErrParse    = -32700 // Invalid JSON received
	mcpErrInvalid  = -32600 // Not a valid JSON-RPC Request object
	mcpErrMethod   = -32601 // Method not found
	mcpErrParams   = -32602 // Invalid method parameters
	mcpErrInternal = -32603 // Internal JSON-RPC error
	mcpErrAuth     = -32001 // Authentication / authorisation failure (server-defined)
)

// ---------------------------------------------------------------------------
// Wire types — JSON-RPC envelope
// ---------------------------------------------------------------------------

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	// ID is absent on notifications; present (possibly null) on requests.
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// mcpResponse is the JSON-RPC response envelope sent back to the client.
// ID must always be serialised (including null), so there is no omitempty here.
type mcpResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *mcpError   `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ---------------------------------------------------------------------------
// MCP method-specific types
// ---------------------------------------------------------------------------

type mcpInitializeResult struct {
	ProtocolVersion string                `json:"protocolVersion"`
	Capabilities    mcpServerCapabilities `json:"capabilities"`
	ServerInfo      mcpImplementation     `json:"serverInfo"`
}

type mcpServerCapabilities struct {
	// Tools signals that this server exposes callable tools.
	Tools *mcpToolsCapability `json:"tools,omitempty"`
}

type mcpToolsCapability struct {
	// ListChanged indicates whether the server emits tools/listChanged notifications.
	// We do not (stateless implementation), so this is always false.
	ListChanged bool `json:"listChanged"`
}

type mcpImplementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type mcpToolsListResult struct {
	Tools []mcpTool `json:"tools"`
}

type mcpTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type mcpToolsCallParams struct {
	Name string `json:"name"`
	// Arguments may be absent — treated as an empty object.
	Arguments json.RawMessage `json:"arguments,omitempty"`
	// Meta carries optional MCP extension fields. PgArachne reads
	// _meta.idempotencyKey and forwards it to pgarachne.save_idempotency_key,
	// identical to the idempotencyKey top-level field on JSON-RPC requests.
	Meta *mcpMeta `json:"_meta,omitempty"`
}

// mcpMeta holds the standard MCP _meta extension block.
type mcpMeta struct {
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

type mcpToolsCallResult struct {
	Content []mcpContent `json:"content"`
	// IsError signals a tool-level error (as opposed to a protocol-level error).
	// The MCP spec requires this field to be present and true when the tool failed.
	IsError bool `json:"isError,omitempty"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// capabilityEntry mirrors one element of the JSON array returned by
// pgarachne.capabilities().
type capabilityEntry struct {
	Method      string          `json:"method"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ---------------------------------------------------------------------------
// Main MCP handler
// ---------------------------------------------------------------------------

// handleMCP is the Streamable HTTP transport endpoint for MCP (POST only).
//
// Routing:
//
//	POST /{prefix}/{database}/mcp
//
// Protocol flow:
//  1. Client calls initialize  → server returns capabilities (no auth required).
//  2. Client sends notifications/initialized → server returns 202 (no body).
//  3. Client calls tools/list  → server calls pgarachne.capabilities() as the
//     authenticated role and maps the result to MCP tool descriptors.
//  4. Client calls tools/call  → server executes schema.function(args::jsonb)
//     as the authenticated role and wraps the result in MCP content blocks.
func (s *Server) handleMCP(c *gin.Context) {
	databaseName := c.Param("database")
	if !isSafeDatabaseName(databaseName) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid database name"})
		return
	}

	var req mcpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, newMCPError(nil, mcpErrParse, "Parse error"))
		return
	}

	if req.JSONRPC != "2.0" {
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInvalid, `Invalid JSON-RPC version, expected "2.0"`))
		return
	}

	// MCP notifications have no "id" field and do not expect a response body.
	// The server MUST return 202 Accepted.
	if isNotification(req) {
		c.Status(http.StatusAccepted)
		return
	}

	// Methods that do not require authentication.
	switch req.Method {
	case "initialize":
		s.handleMCPInitialize(c, req)
		return
	case "ping":
		c.JSON(http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: struct{}{}})
		return
	}

	// All remaining methods require a valid database connection and authentication.
	db, err := database.GetConnection(s.Cfg, databaseName)
	if err != nil {
		slog.Error("MCP: database connection failed", "database", databaseName, "error", err)
		c.JSON(http.StatusServiceUnavailable, newMCPError(req.ID, mcpErrInternal, "Database connection failed"))
		return
	}

	dbRole, errMsg, httpStatus := s.authenticateToken(c, db, databaseName)
	if errMsg != "" {
		c.JSON(httpStatus, newMCPError(req.ID, mcpErrAuth, errMsg))
		return
	}

	switch req.Method {
	case "tools/list":
		s.handleMCPToolsList(c, req, db, dbRole)
	case "tools/call":
		s.handleMCPToolsCall(c, req, db, dbRole)
	default:
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrMethod, "Method not found: "+req.Method))
	}
}

// ---------------------------------------------------------------------------
// Method handlers
// ---------------------------------------------------------------------------

// handleMCPInitialize handles the MCP initialize handshake.
// Authentication is NOT required — the client needs to discover capabilities
// before it can present credentials.
func (s *Server) handleMCPInitialize(c *gin.Context, req mcpRequest) {
	c.JSON(http.StatusOK, mcpResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: mcpInitializeResult{
			ProtocolVersion: mcpProtocolVersion,
			Capabilities: mcpServerCapabilities{
				Tools: &mcpToolsCapability{ListChanged: false},
			},
			ServerInfo: mcpImplementation{
				Name:    "PgArachne",
				Version: "1.0.0",
			},
		},
	})
}

// handleMCPToolsList handles tools/list by calling pgarachne.capabilities()
// as the authenticated role and mapping the result to MCP tool descriptors.
func (s *Server) handleMCPToolsList(c *gin.Context, req mcpRequest, db *sql.DB, dbRole string) {
	caps, err := s.fetchCapabilitiesForRole(c.Request.Context(), db, dbRole)
	if err != nil {
		slog.Error("MCP tools/list: capabilities fetch failed", "error", err)
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInternal, "Failed to list tools"))
		return
	}

	tools := make([]mcpTool, 0, len(caps))
	for _, cap := range caps {
		// Fall back to a minimal valid JSON Schema when the function has no
		// parameter declaration (e.g., functions that only accept an empty object).
		inputSchema := cap.Parameters
		if len(inputSchema) == 0 {
			inputSchema = json.RawMessage(`{"type":"object","properties":{}}`)
		}

		tools = append(tools, mcpTool{
			Name:        cap.Method,
			Description: cap.Description,
			InputSchema: inputSchema,
		})
	}

	c.JSON(http.StatusOK, mcpResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpToolsListResult{Tools: tools},
	})
}

// handleMCPToolsCall handles tools/call by executing the named PostgreSQL
// function inside a transaction with SET LOCAL ROLE, then wrapping the JSON
// result in an MCP content block.
//
// Tool execution failures (e.g., function not found, SQL error) are returned
// as tool-level errors (IsError: true in the result), NOT as JSON-RPC errors,
// per MCP specification §tool-result.
func (s *Server) handleMCPToolsCall(c *gin.Context, req mcpRequest, db *sql.DB, dbRole string) {
	if req.Params == nil {
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrParams, "params is required for tools/call"))
		return
	}

	var params mcpToolsCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrParams, "Invalid params: "+err.Error()))
		return
	}

	functionName := strings.TrimSpace(params.Name)
	if functionName == "" {
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrParams, "Tool name is required"))
		return
	}
	if !isSafeFunctionName(functionName) {
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrParams, "Invalid tool name"))
		return
	}

	// Treat absent arguments as an empty JSON object so the SQL function
	// always receives a valid jsonb value.
	argsJSON := params.Arguments
	if len(argsJSON) == 0 {
		argsJSON = json.RawMessage(`{}`)
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		slog.Error("MCP tools/call: begin tx failed", "function", functionName, "error", err)
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInternal, "Database unavailable"))
		return
	}
	defer tx.Rollback()

	// Idempotency check — identical semantics to the JSON-RPC endpoint.
	// Runs as the service user (DB_USER) before SET LOCAL ROLE, so no extra
	// privileges are needed on the client role. The check is inside the
	// transaction: if the function call fails and the tx rolls back, the key
	// is not persisted and the client may retry.
	if params.Meta != nil && params.Meta.IdempotencyKey != "" {
		var saved bool
		err := tx.QueryRowContext(
			c.Request.Context(),
			`SELECT pgarachne.save_idempotency_key($1)`,
			params.Meta.IdempotencyKey,
		).Scan(&saved)
		if err != nil {
			slog.Error("MCP tools/call: idempotency check failed",
				"key", params.Meta.IdempotencyKey, "function", functionName, "error", err)
			recordJSONRPC(functionName, "error")
			c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInternal, "Idempotency check failed"))
			return
		}
		if !saved {
			slog.Warn("MCP tools/call: duplicate request rejected",
				"key", params.Meta.IdempotencyKey, "function", functionName)
			recordJSONRPC(functionName, "duplicate")
			c.JSON(http.StatusOK, mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpToolsCallResult{
					Content: []mcpContent{{Type: "text", Text: "This request has already been processed"}},
					IsError: true,
				},
			})
			return
		}
	}

	// Impersonate the authenticated role for the duration of the transaction.
	// The double-quote escaping mirrors the pattern used in handleFunctionCall.
	quotedRole := fmt.Sprintf(`"%s"`, strings.ReplaceAll(dbRole, `"`, `""`))
	if _, err := tx.ExecContext(c.Request.Context(), "SET LOCAL ROLE "+quotedRole); err != nil {
		slog.Error("MCP tools/call: SET LOCAL ROLE failed", "role", dbRole, "function", functionName, "error", err)
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrAuth, "Permission denied for the specified role"))
		return
	}

	var query string
	if functionName == "capabilities" || functionName == "pgarachne.capabilities" {
		query = `SELECT pgarachne.capabilities($1::jsonb)::json`
	} else {
		query = fmt.Sprintf("SELECT %s($1::jsonb)::json", functionName)
	}

	var resultJSON json.RawMessage
	if err := tx.QueryRowContext(c.Request.Context(), query, []byte(argsJSON)).Scan(&resultJSON); err != nil {
		slog.Error("MCP tools/call: function execution failed", "function", functionName, "error", err)
		recordJSONRPC(functionName, "error")

		// Return a tool-level error, not a JSON-RPC protocol error.
		// Per MCP spec, SQL failures are tool failures, not transport failures.
		errText := fmt.Sprintf("Function call failed: %s", sanitiseSQLError(err))
		c.JSON(http.StatusOK, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpToolsCallResult{
				Content: []mcpContent{{Type: "text", Text: errText}},
				IsError: true,
			},
		})
		return
	}

	if err := tx.Commit(); err != nil {
		slog.Error("MCP tools/call: commit failed", "function", functionName, "error", err)
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInternal, "Transaction commit failed"))
		return
	}

	recordJSONRPC(functionName, "success")

	c.JSON(http.StatusOK, mcpResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: mcpToolsCallResult{
			Content: []mcpContent{{Type: "text", Text: mcpFormatResult(resultJSON)}},
		},
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// fetchCapabilitiesForRole runs pgarachne.capabilities() inside a short
// transaction with SET LOCAL ROLE so that has_function_privilege() in the
// query returns results appropriate for the authenticated user.
func (s *Server) fetchCapabilitiesForRole(ctx context.Context, db *sql.DB, dbRole string) ([]capabilityEntry, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	quotedRole := fmt.Sprintf(`"%s"`, strings.ReplaceAll(dbRole, `"`, `""`))
	if _, err := tx.ExecContext(ctx, "SET LOCAL ROLE "+quotedRole); err != nil {
		return nil, fmt.Errorf("set role: %w", err)
	}

	var raw json.RawMessage
	if err := tx.QueryRowContext(ctx, `SELECT pgarachne.capabilities($1::jsonb)::json`, `{}`).Scan(&raw); err != nil {
		return nil, fmt.Errorf("capabilities query: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	var entries []capabilityEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("unmarshal capabilities: %w", err)
	}
	return entries, nil
}

// isNotification reports whether req is an MCP notification.
// Notifications never carry a response: the server must return 202 Accepted.
// A message is a notification when it has no "id" field (ID is nil after
// unmarshal) OR when the method name is in the notifications/ namespace.
func isNotification(req mcpRequest) bool {
	return req.ID == nil || strings.HasPrefix(req.Method, "notifications/")
}

// newMCPError builds a JSON-RPC error response.
func newMCPError(id interface{}, code int, message string) mcpResponse {
	return mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &mcpError{Code: code, Message: message},
	}
}

// mcpFormatResult pretty-prints a JSON value for inclusion in an MCP text
// content block. Falls back to the raw bytes when the value cannot be
// re-marshalled.
func mcpFormatResult(raw json.RawMessage) string {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(pretty)
}

// sanitiseSQLError returns the error message with driver-level prefixes
// stripped so that tool-level error messages are readable.
func sanitiseSQLError(err error) string {
	msg := err.Error()
	// pq errors look like "pq: <message>"; strip the prefix for cleaner output.
	if strings.HasPrefix(msg, "pq: ") {
		return strings.TrimPrefix(msg, "pq: ")
	}
	return msg
}
