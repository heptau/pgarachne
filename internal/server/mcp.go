package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/heptau/pgarachne/internal/database"
	"github.com/heptau/pgarachne/internal/version"
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

// mcpDatabaseMethods maps MCP protocol method names to their pgarachne SQL
// backing functions. Each function accepts a jsonb params argument and returns
// json shaped according to the MCP specification for that method.
//
// Adding a new MCP method backed by a SQL function only requires:
//  1. Implementing the function in sql/mcp_functions.sql.
//  2. Adding an entry here — no other Go changes are needed.
var mcpDatabaseMethods = map[string]string{
	"resources/list": "pgarachne.mcp_list_resources",
	"resources/read": "pgarachne.mcp_read_resource",
	"prompts/list":   "pgarachne.mcp_list_prompts",
	"prompts/get":    "pgarachne.mcp_get_prompt",
}

// ---------------------------------------------------------------------------
// Wire types — JSON-RPC envelope
// ---------------------------------------------------------------------------

type mcpRequest struct {
	JSONRPC string `json:"jsonrpc"`
	// ID is absent on notifications; present (possibly null) on requests.
	ID     interface{}     `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
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
	// Tools signals that this server exposes callable tools (PostgreSQL functions).
	Tools *mcpToolsCapability `json:"tools,omitempty"`
	// Resources signals that this server exposes readable resources (tables/views).
	Resources *mcpResourcesCapability `json:"resources,omitempty"`
	// Prompts signals that this server exposes prompt templates.
	Prompts *mcpPromptsCapability `json:"prompts,omitempty"`
}

type mcpToolsCapability struct {
	// ListChanged — we do not emit tools/listChanged notifications (stateless).
	ListChanged bool `json:"listChanged"`
}

// mcpResourcesCapability describes the server's resources support.
// Subscribe: we do not implement live resource subscriptions (no SSE per resource).
// ListChanged: we do not emit resources/listChanged notifications.
type mcpResourcesCapability struct {
	Subscribe   bool `json:"subscribe"`
	ListChanged bool `json:"listChanged"`
}

// mcpPromptsCapability describes the server's prompts support.
// ListChanged: we do not emit prompts/listChanged notifications.
type mcpPromptsCapability struct {
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
//  1. Client calls initialize     → server returns capabilities (no auth required).
//  2. Client sends notifications/ → server returns 202 Accepted (no body).
//  3. Client calls tools/list     → pgarachne.capabilities() as authenticated role.
//  4. Client calls tools/call     → schema.function(args::jsonb) as authenticated role.
//  5. Client calls resources/list → pgarachne.mcp_list_resources() as authenticated role.
//  6. Client calls resources/read → pgarachne.mcp_read_resource(params) as authenticated role.
//  7. Client calls prompts/list   → pgarachne.mcp_list_prompts() as authenticated role.
//  8. Client calls prompts/get    → pgarachne.mcp_get_prompt(params) as authenticated role.
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
	// The server MUST return 202 Accepted with no body.
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
	// Three modes — mirrors the JSON-RPC endpoint:
	//   Basic Auth  → direct user pool, dbRole = "" (no SET LOCAL ROLE).
	//   Bearer JWT  → system pool, dbRole = JWT subject.
	//   API token   → system pool, dbRole = token's role.
	var db *sql.DB
	var dbRole string

	if username, password, ok := parseBasicAuth(c.GetHeader("Authorization")); ok {
		if len(username) == 0 || len(username) > MaxLoginLength || len(password) > MaxPasswordLength {
			recordAuthResult("direct", "malformed")
			c.JSON(http.StatusUnauthorized, newMCPError(req.ID, mcpErrAuth, "Invalid credentials"))
			return
		}
		userDB, err := database.GetUserConnection(s.Cfg, databaseName, username, password)
		if err != nil {
			slog.Warn("MCP: direct authentication failed", "user", username, "database", databaseName, "error", err)
			recordAuthResult("direct", "invalid")
			c.JSON(http.StatusUnauthorized, newMCPError(req.ID, mcpErrAuth, "Invalid credentials"))
			return
		}
		recordAuthResult("direct", "success")
		db = userDB
		// dbRole = "" → SET LOCAL ROLE is skipped in sub-handlers.
	} else {
		sysDB, err := database.GetConnection(s.Cfg, databaseName)
		if err != nil {
			slog.Error("MCP: database connection failed", "database", databaseName, "error", err)
			c.JSON(http.StatusServiceUnavailable, newMCPError(req.ID, mcpErrInternal, "Database connection failed"))
			return
		}
		role, errMsg, httpStatus := s.authenticateToken(c, sysDB, databaseName)
		if errMsg != "" {
			c.JSON(httpStatus, newMCPError(req.ID, mcpErrAuth, errMsg))
			return
		}
		db = sysDB
		dbRole = role
	}

	// tools/* methods are handled explicitly because tools/call has special
	// logic (idempotency, tool-level error wrapping, argument extraction).
	switch req.Method {
	case "tools/list":
		s.handleMCPToolsList(c, req, db, dbRole)
		return
	case "tools/call":
		s.handleMCPToolsCall(c, req, db, dbRole)
		return
	}

	// All other authenticated methods are backed by a pgarachne SQL function
	// registered in mcpDatabaseMethods. This covers resources/* and prompts/*,
	// and can be extended without adding new Go code.
	if sqlFunc, ok := mcpDatabaseMethods[req.Method]; ok {
		s.handleMCPDatabaseMethod(c, req, db, dbRole, sqlFunc)
		return
	}

	slog.Warn("MCP: unknown method", "method", req.Method, "database", databaseName)
	c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrMethod, "Method not found: "+req.Method))
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
				Tools: &mcpToolsCapability{
					ListChanged: false,
				},
				Resources: &mcpResourcesCapability{
					Subscribe:   false,
					ListChanged: false,
				},
				Prompts: &mcpPromptsCapability{
					ListChanged: false,
				},
			},
			ServerInfo: mcpImplementation{
				Name:    "PgArachne",
				Version: version.Version,
			},
		},
	})
}

// handleMCPToolsList handles tools/list by calling pgarachne.capabilities()
// as the authenticated role and mapping the result to MCP tool descriptors.
func (s *Server) handleMCPToolsList(c *gin.Context, req mcpRequest, db *sql.DB, dbRole string) {
	raw, err := s.callPgarachneFunc(c.Request.Context(), db, dbRole,
		"pgarachne.capabilities", json.RawMessage(`{}`))
	if err != nil {
		slog.Error("MCP tools/list: capabilities fetch failed", "error", err)
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInternal, "Failed to list tools"))
		return
	}

	var caps []capabilityEntry
	if err := json.Unmarshal(raw, &caps); err != nil {
		slog.Error("MCP tools/list: failed to unmarshal capabilities", "error", err)
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInternal, "Failed to parse capabilities"))
		return
	}

	tools := make([]mcpTool, 0, len(caps))
	for _, cap := range caps {
		// Fall back to a minimal valid JSON Schema when the function has no
		// parameter declaration.
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
	if len(functionName) > MaxMethodLength {
		recordJSONRPC("", "error")
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrParams, "Tool name is too long"))
		return
	}
	if !isSafeFunctionName(functionName) {
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrParams, "Invalid tool name"))
		return
	}
	if params.Meta != nil && len(params.Meta.IdempotencyKey) > MaxIdempotencyKeyLength {
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrParams, "idempotencyKey is too long"))
		return
	}

	// Treat absent or null arguments as an empty JSON object so the SQL
	// function always receives a valid jsonb value.
	argsJSON := params.Arguments
	if len(argsJSON) == 0 || string(argsJSON) == "null" {
		argsJSON = json.RawMessage(`{}`)
	}

	tx, err := db.BeginTx(c.Request.Context(), nil)
	if err != nil {
		slog.Error("MCP tools/call: begin tx failed", "function", functionName, "error", err)
		recordJSONRPC(functionName, "error")
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInternal, "Database unavailable"))
		return
	}
	defer rollbackQuietly(tx)

	idempotencyKey := ""
	if params.Meta != nil {
		idempotencyKey = params.Meta.IdempotencyKey
	}

	// Shared setup with the JSON-RPC endpoint: API prefix, idempotency check,
	// SET LOCAL ROLE — see setupRequestTx for ordering and privilege notes.
	// Per MCP spec, a duplicate request is a tool-level error; the other
	// failures are protocol errors.
	if err := s.setupRequestTx(c.Request.Context(), tx, dbRole, idempotencyKey); err != nil {
		switch {
		case errors.Is(err, errIdempotencyDuplicate):
			slog.Warn("MCP tools/call: duplicate request rejected",
				"key", idempotencyKey, "function", functionName)
			recordJSONRPC(functionName, "duplicate")
			c.JSON(http.StatusOK, mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpToolsCallResult{
					Content: []mcpContent{{Type: "text", Text: "This request has already been processed"}},
					IsError: true,
				},
			})
		case errors.Is(err, errIdempotencyCheckFailed):
			slog.Error("MCP tools/call: idempotency check failed",
				"key", idempotencyKey, "function", functionName, "error", err)
			recordJSONRPC(functionName, "error")
			c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInternal, "Idempotency check failed"))
		default:
			slog.Error("MCP tools/call: SET LOCAL ROLE failed", "role", dbRole, "function", functionName, "error", err)
			recordJSONRPC(functionName, "error")
			c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrAuth, "Permission denied for the specified role"))
		}
		return
	}

	// functionName is part of SQL syntax (it names the function to call), not
	// a value, so it can't be passed as a bind parameter — isSafeFunctionName
	// above is the mitigation instead, requiring a strict schema.function
	// identifier shape before functionName ever reaches buildFunctionQuery.
	query := buildFunctionQuery(functionName)

	var resultJSON json.RawMessage
	if err := tx.QueryRowContext(c.Request.Context(), query, []byte(argsJSON)).Scan(&resultJSON); err != nil { // codeql[go/sql-injection] -- functionName is validated by isSafeFunctionName() before reaching here
		slog.Error("MCP tools/call: function execution failed", "function", functionName, "error", err)
		recordJSONRPC(functionName, "error")

		// Return a tool-level error, not a JSON-RPC protocol error.
		// Per MCP spec, SQL failures are tool failures, not transport failures.
		errText := s.sqlErrorText(err)
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

// handleMCPDatabaseMethod is the generic handler for MCP methods backed by a
// pgarachne SQL function (resources/list, resources/read, prompts/list,
// prompts/get, and any future extensions registered in mcpDatabaseMethods).
//
// Unlike tools/call, failures here are returned as JSON-RPC protocol errors
// (with the error field set) rather than tool-level errors, because these
// methods represent infrastructure queries — a missing resource or prompt is a
// caller error, not a tool execution failure.
func (s *Server) handleMCPDatabaseMethod(c *gin.Context, req mcpRequest, db *sql.DB, dbRole, sqlFunc string) {
	params := req.Params
	if len(params) == 0 {
		params = json.RawMessage(`{}`)
	}

	raw, err := s.callPgarachneFunc(c.Request.Context(), db, dbRole, sqlFunc, params)
	if err != nil {
		slog.Error("MCP database method failed",
			"method", req.Method, "func", sqlFunc, "error", err)
		errText := "Method execution failed"
		if s.Cfg.MCPSQLErrorDetail {
			errText = sanitiseSQLError(err)
		}
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInternal, errText))
		return
	}

	// The SQL function already returns a fully-shaped MCP result object.
	// Unmarshal to interface{} so it is re-serialised without double encoding.
	var result interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		slog.Error("MCP database method: failed to unmarshal result",
			"method", req.Method, "func", sqlFunc, "error", err)
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInternal, "Failed to parse method result"))
		return
	}

	c.JSON(http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: result})
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// callPgarachneFunc executes a pgarachne SQL function inside a transaction
// with SET LOCAL ROLE applied. The function must accept a single jsonb
// argument and return json.
//
// This is the shared execution primitive used by:
//   - handleMCPToolsList  (via capabilities)
//   - handleMCPDatabaseMethod (resources/*, prompts/*)
func (s *Server) callPgarachneFunc(
	ctx context.Context,
	db *sql.DB,
	dbRole string,
	sqlFunc string,
	params json.RawMessage,
) (json.RawMessage, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer rollbackQuietly(tx)

	// Shared setup with the JSON-RPC endpoint (API prefix + SET LOCAL ROLE);
	// no idempotency key for infrastructure queries.
	if err := s.setupRequestTx(ctx, tx, dbRole, ""); err != nil {
		return nil, fmt.Errorf("set role %s: %w", dbRole, err)
	}

	var raw json.RawMessage
	if err := tx.QueryRowContext(
		ctx,
		fmt.Sprintf("SELECT %s($1::jsonb)::json", sqlFunc),
		[]byte(params),
	).Scan(&raw); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return raw, nil
}

// isNotification reports whether req is an MCP notification.
// Notifications never carry a response: the server must return 202 Accepted
// with no body. Per the MCP spec, a message is a notification when its method
// name starts with "notifications/". The "id" field is intentionally NOT used
// for this classification: a request with id:null is still a request and the
// client expects a JSON-RPC response (whose id is also null).
func isNotification(req mcpRequest) bool {
	return strings.HasPrefix(req.Method, "notifications/")
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

// sqlErrorText returns the text shown to MCP clients for a failed tool call.
// With MCPSQLErrorDetail enabled it includes the PostgreSQL error message,
// which helps LLM agents self-correct (wrong argument names, missing values).
// Disabled (the default), it returns a generic message matching the JSON-RPC
// endpoint, so authenticated callers cannot harvest schema details (table and
// constraint names, RAISE text) from error responses.
func (s *Server) sqlErrorText(err error) string {
	if !s.Cfg.MCPSQLErrorDetail {
		return "Function call failed"
	}
	return "Function call failed: " + sanitiseSQLError(err)
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
