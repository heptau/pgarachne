package server

import (
	"context"
	"database/sql"
	"encoding/base64"
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

// mcpProtocolVersion is the MCP protocol version this server implements.
// PgArachne targets the stateless, per-request protocol introduced in
// 2026-07-28 exclusively — there is no dual-era support for the older
// initialize-handshake versions (2025-11-25 and earlier).
const mcpProtocolVersion = "2026-07-28"

const mcpServerName = "PgArachne"

// Well-known _meta keys defined by the MCP specification. Every request
// carries protocolVersion + clientCapabilities; every result should carry
// serverInfo.
const (
	mcpMetaProtocolVersion    = "io.modelcontextprotocol/protocolVersion"
	mcpMetaClientCapabilities = "io.modelcontextprotocol/clientCapabilities"
	mcpMetaClientInfo         = "io.modelcontextprotocol/clientInfo"
	mcpMetaServerInfo         = "io.modelcontextprotocol/serverInfo"
)

// resultType values. PgArachne never needs additional client input mid-call
// (no sampling/elicitation/roots), so every result it returns is "complete".
const mcpResultTypeComplete = "complete"

// cacheScope values for CacheableResult fields.
const (
	mcpCacheScopePublic  = "public"
	mcpCacheScopePrivate = "private"
)

// Freshness hints (ttlMs) for cacheable results. resources/read returns live
// table data, so it is not cacheable at all. The others vary by the
// authenticated role's grants, so they are scoped "private" rather than
// "public" even though they are given a TTL.
const (
	mcpDiscoverTTLMs      = 3_600_000 // 1 hour — static server identity/capabilities
	mcpToolsListTTLMs     = 60_000    // 1 minute
	mcpResourcesListTTLMs = 60_000    // 1 minute
	mcpResourcesReadTTLMs = 0         // live data, do not cache
	mcpPromptsListTTLMs   = 300_000   // 5 minutes
)

// JSON-RPC 2.0 error codes mandated by the MCP specification.
const (
	mcpErrParse    = -32700 // Invalid JSON received
	mcpErrInvalid  = -32600 // Not a valid JSON-RPC Request object
	mcpErrMethod   = -32601 // Method not found
	mcpErrParams   = -32602 // Invalid method parameters
	mcpErrInternal = -32603 // Internal JSON-RPC error

	// mcpErrAuth is allocated from the legacy implementation-defined range
	// (-32000 to -32019), which the 2026-07-28 spec grandfathers for
	// implementations that allocated codes before the range was formalised.
	mcpErrAuth = -32001 // Authentication / authorisation failure (server-defined)

	// Codes below are allocated by the MCP specification itself, from the
	// range it reserves (-32020 to -32099).
	mcpErrHeaderMismatch     = -32020 // Mcp-* headers disagree with the request body
	mcpErrUnsupportedVersion = -32022 // Requested protocolVersion is not supported
)

// mcpDatabaseMethods maps MCP protocol method names to their pgarachne SQL
// backing functions, plus the CacheableResult hints each method's result
// should carry (2026-07-28 requires ttlMs/cacheScope on tools/list,
// prompts/list, resources/list and resources/read, but not on prompts/get).
//
// Adding a new MCP method backed by a SQL function only requires:
//  1. Implementing the function in sql/mcp_functions.sql.
//  2. Adding an entry here — no other Go changes are needed.
var mcpDatabaseMethods = map[string]mcpDatabaseMethod{
	"resources/list": {sqlFunc: "pgarachne.mcp_list_resources", cacheable: true, ttlMs: mcpResourcesListTTLMs, cacheScope: mcpCacheScopePrivate},
	"resources/read": {sqlFunc: "pgarachne.mcp_read_resource", cacheable: true, ttlMs: mcpResourcesReadTTLMs, cacheScope: mcpCacheScopePrivate},
	"prompts/list":   {sqlFunc: "pgarachne.mcp_list_prompts", cacheable: true, ttlMs: mcpPromptsListTTLMs, cacheScope: mcpCacheScopePrivate},
	"prompts/get":    {sqlFunc: "pgarachne.mcp_get_prompt", cacheable: false},
}

// mcpDatabaseMethod describes one entry in mcpDatabaseMethods.
type mcpDatabaseMethod struct {
	sqlFunc    string
	cacheable  bool
	ttlMs      int64
	cacheScope string
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
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// mcpMeta holds the standard MCP _meta extension block. It combines the
// protocol's own per-request fields with PgArachne's idempotency extension —
// both live under the same "_meta" key in the wire format.
type mcpMeta struct {
	IdempotencyKey     string          `json:"idempotencyKey,omitempty"`
	ProtocolVersion    string          `json:"io.modelcontextprotocol/protocolVersion,omitempty"`
	ClientCapabilities json.RawMessage `json:"io.modelcontextprotocol/clientCapabilities,omitempty"`
	ClientInfo         json.RawMessage `json:"io.modelcontextprotocol/clientInfo,omitempty"`
}

// mcpParamsBase captures the fields common to every MCP request's params
// object. It is used for transport-level validation (protocol version,
// header agreement) before method-specific dispatch; individual methods
// still declare their own stricter params types for the fields they use.
type mcpParamsBase struct {
	Meta *mcpMeta `json:"_meta"`
	Name string   `json:"name"`
	URI  string   `json:"uri"`
}

// ---------------------------------------------------------------------------
// MCP method-specific types
// ---------------------------------------------------------------------------

type mcpDiscoverResult struct {
	SupportedVersions []string              `json:"supportedVersions"`
	Capabilities      mcpServerCapabilities `json:"capabilities"`
	Instructions      string                `json:"instructions,omitempty"`
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
	// ListChanged — we do not emit tools/list_changed notifications (stateless).
	ListChanged bool `json:"listChanged"`
}

// mcpResourcesCapability describes the server's resources support.
// Subscribe: we do not implement subscriptions/listen.
// ListChanged: we do not emit resources/list_changed notifications.
type mcpResourcesCapability struct {
	Subscribe   bool `json:"subscribe"`
	ListChanged bool `json:"listChanged"`
}

// mcpPromptsCapability describes the server's prompts support.
// ListChanged: we do not emit prompts/list_changed notifications.
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
	// Meta carries the standard MCP _meta extension block — both the
	// protocol's per-request fields and PgArachne's idempotencyKey.
	Meta *mcpMeta `json:"_meta,omitempty"`
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
// Protocol flow (2026-07-28, stateless — no initialize/session handshake):
//  1. Every request must carry params._meta.protocolVersion +
//     clientCapabilities, and the MCP-Protocol-Version / Mcp-Method / Mcp-Name
//     HTTP headers must agree with the request body.
//  2. Client calls server/discover → server returns supported versions,
//     capabilities, identity (no auth required).
//  3. Client sends notifications/*    → server returns 202 Accepted (no body).
//  4. Client calls tools/list         → pgarachne.capabilities() as authenticated role.
//  5. Client calls tools/call         → schema.function(args::jsonb) as authenticated role.
//  6. Client calls resources/list     → pgarachne.mcp_list_resources() as authenticated role.
//  7. Client calls resources/read     → pgarachne.mcp_read_resource(params) as authenticated role.
//  8. Client calls prompts/list       → pgarachne.mcp_list_prompts() as authenticated role.
//  9. Client calls prompts/get        → pgarachne.mcp_get_prompt(params) as authenticated role.
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

	// Transport-level validation introduced by protocol version 2026-07-28:
	// every request must declare its protocol version and client
	// capabilities, and the standard Mcp-* headers must agree with the body.
	if errResp := mcpValidateRequest(c, req); errResp != nil {
		c.JSON(http.StatusBadRequest, *errResp)
		return
	}

	// Methods that do not require authentication.
	if req.Method == "server/discover" {
		s.handleMCPDiscover(c, req)
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
	if method, ok := mcpDatabaseMethods[req.Method]; ok {
		s.handleMCPDatabaseMethod(c, req, db, dbRole, method)
		return
	}

	slog.Warn("MCP: unknown method", "method", req.Method, "database", databaseName)
	c.JSON(http.StatusNotFound, newMCPError(req.ID, mcpErrMethod, "Method not found: "+req.Method))
}

// mcpMethodNotAllowed handles GET and DELETE on the MCP endpoint. Protocol
// versions 2025-03-26 through 2025-11-25 used GET to open a standalone SSE
// stream and DELETE to terminate a session; 2026-07-28 removed both along
// with protocol-level sessions, and directs servers that only implement this
// revision to reject such requests with 405.
func mcpMethodNotAllowed(c *gin.Context) {
	c.Status(http.StatusMethodNotAllowed)
}

// ---------------------------------------------------------------------------
// Transport-level validation (2026-07-28)
// ---------------------------------------------------------------------------

// mcpValidateRequest enforces the transport requirements introduced in MCP
// 2026-07-28: every request must declare its protocol version and client
// capabilities in params._meta, the MCP-Protocol-Version header must agree
// with that _meta field and be a version this server supports, and the
// Mcp-Method (and, for name/uri-addressed methods, Mcp-Name) headers must
// agree with the request body. It returns a non-nil error response — to be
// sent with HTTP 400 — when validation fails.
func mcpValidateRequest(c *gin.Context, req mcpRequest) *mcpResponse {
	var params mcpParamsBase
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp := newMCPError(req.ID, mcpErrParams, "Invalid params: "+err.Error())
			return &resp
		}
	}

	if params.Meta == nil || params.Meta.ProtocolVersion == "" || params.Meta.ClientCapabilities == nil {
		resp := newMCPError(req.ID, mcpErrParams,
			`params._meta must include "`+mcpMetaProtocolVersion+`" and "`+mcpMetaClientCapabilities+`"`)
		return &resp
	}
	metaVersion := params.Meta.ProtocolVersion

	headerVersion := c.GetHeader("MCP-Protocol-Version")
	if headerVersion == "" {
		resp := newMCPError(req.ID, mcpErrHeaderMismatch, "Missing required header: MCP-Protocol-Version")
		return &resp
	}
	if headerVersion != metaVersion {
		resp := newMCPError(req.ID, mcpErrHeaderMismatch,
			fmt.Sprintf("Header mismatch: MCP-Protocol-Version header value %q does not match body value %q", headerVersion, metaVersion))
		return &resp
	}
	if metaVersion != mcpProtocolVersion {
		return mcpUnsupportedVersionError(req.ID, metaVersion)
	}

	methodHeader := c.GetHeader("Mcp-Method")
	if methodHeader == "" {
		resp := newMCPError(req.ID, mcpErrHeaderMismatch, "Missing required header: Mcp-Method")
		return &resp
	}
	if methodHeader != req.Method {
		resp := newMCPError(req.ID, mcpErrHeaderMismatch,
			fmt.Sprintf("Header mismatch: Mcp-Method header value %q does not match body method %q", methodHeader, req.Method))
		return &resp
	}

	if requiresNameHeader(req.Method) {
		expected := params.Name
		if req.Method == "resources/read" {
			expected = params.URI
		}
		nameHeader, err := decodeMCPHeaderValue(c.GetHeader("Mcp-Name"))
		if err != nil {
			resp := newMCPError(req.ID, mcpErrHeaderMismatch, "Invalid Mcp-Name header encoding")
			return &resp
		}
		if nameHeader == "" {
			resp := newMCPError(req.ID, mcpErrHeaderMismatch, "Missing required header: Mcp-Name")
			return &resp
		}
		if nameHeader != expected {
			resp := newMCPError(req.ID, mcpErrHeaderMismatch,
				fmt.Sprintf("Header mismatch: Mcp-Name header value %q does not match body value %q", nameHeader, expected))
			return &resp
		}
	}

	return nil
}

// requiresNameHeader reports whether method requires the Mcp-Name header,
// per the Streamable HTTP standard request headers table.
func requiresNameHeader(method string) bool {
	switch method {
	case "tools/call", "resources/read", "prompts/get":
		return true
	default:
		return false
	}
}

// decodeMCPHeaderValue decodes the "=?base64?...?=" sentinel format used to
// carry header values that cannot be represented as plain ASCII. Values
// without the sentinel are returned unchanged.
func decodeMCPHeaderValue(v string) (string, error) {
	const prefix = "=?base64?"
	const suffix = "?="
	if v == "" || !strings.HasPrefix(v, prefix) || !strings.HasSuffix(v, suffix) {
		return v, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(v[len(prefix) : len(v)-len(suffix)])
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// mcpUnsupportedVersionError builds the UnsupportedProtocolVersionError
// response mandated when a request's protocol version is not supported.
func mcpUnsupportedVersionError(id interface{}, requested string) *mcpResponse {
	data, err := json.Marshal(struct {
		Supported []string `json:"supported"`
		Requested string   `json:"requested"`
	}{
		Supported: []string{mcpProtocolVersion},
		Requested: requested,
	})
	if err != nil {
		data = nil
	}
	return &mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcpError{
			Code:    mcpErrUnsupportedVersion,
			Message: "Unsupported protocol version",
			Data:    data,
		},
	}
}

// ---------------------------------------------------------------------------
// Method handlers
// ---------------------------------------------------------------------------

// handleMCPDiscover handles server/discover, the 2026-07-28 replacement for
// the old initialize handshake. Authentication is NOT required — clients may
// call this before presenting credentials to learn what the server supports.
func (s *Server) handleMCPDiscover(c *gin.Context, req mcpRequest) {
	result, err := mcpWrapCacheableResult(mcpDiscoverResult{
		SupportedVersions: []string{mcpProtocolVersion},
		Capabilities: mcpServerCapabilities{
			Tools:     &mcpToolsCapability{ListChanged: false},
			Resources: &mcpResourcesCapability{Subscribe: false, ListChanged: false},
			Prompts:   &mcpPromptsCapability{ListChanged: false},
		},
		Instructions: "PgArachne exposes PostgreSQL functions as MCP tools, tables and views as MCP resources, " +
			"and stored templates as MCP prompts. Authenticate with a Bearer token (JWT or long-lived API token) " +
			"or HTTP Basic credentials.",
	}, mcpDiscoverTTLMs, mcpCacheScopePublic)
	if err != nil {
		slog.Error("MCP server/discover: failed to build result", "error", err)
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInternal, "Failed to build discover result"))
		return
	}
	c.JSON(http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: result})
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

	result, err := mcpWrapCacheableResult(mcpToolsListResult{Tools: tools}, mcpToolsListTTLMs, mcpCacheScopePrivate)
	if err != nil {
		slog.Error("MCP tools/list: failed to build result", "error", err)
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInternal, "Failed to list tools"))
		return
	}
	c.JSON(http.StatusOK, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: result})
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
				Result:  mcpToolResult([]mcpContent{{Type: "text", Text: "This request has already been processed"}}, true),
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
	// See the matching comment in handleFunctionCall (server.go) for why
	// CodeQL's go/sql-injection alert on this line is a false positive.
	query := buildFunctionQuery(functionName)

	var resultJSON json.RawMessage
	if err := tx.QueryRowContext(c.Request.Context(), query, []byte(argsJSON)).Scan(&resultJSON); err != nil {
		slog.Error("MCP tools/call: function execution failed", "function", functionName, "error", err)
		recordJSONRPC(functionName, "error")

		// Return a tool-level error, not a JSON-RPC protocol error.
		// Per MCP spec, SQL failures are tool failures, not transport failures.
		errText := s.sqlErrorText(err)
		c.JSON(http.StatusOK, mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpToolResult([]mcpContent{{Type: "text", Text: errText}}, true),
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
		Result:  mcpToolResult([]mcpContent{{Type: "text", Text: mcpFormatResult(resultJSON)}}, false),
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
func (s *Server) handleMCPDatabaseMethod(c *gin.Context, req mcpRequest, db *sql.DB, dbRole string, method mcpDatabaseMethod) {
	params := req.Params
	if len(params) == 0 {
		params = json.RawMessage(`{}`)
	}

	raw, err := s.callPgarachneFunc(c.Request.Context(), db, dbRole, method.sqlFunc, params)
	if err != nil {
		slog.Error("MCP database method failed",
			"method", req.Method, "func", method.sqlFunc, "error", err)
		errText := "Method execution failed"
		if s.Cfg.MCPSQLErrorDetail {
			errText = sanitiseSQLError(err)
		}
		c.JSON(http.StatusOK, newMCPError(req.ID, mcpErrInternal, errText))
		return
	}

	// The SQL function already returns a fully-shaped MCP result object; wrap
	// it with the resultType/_meta/cache fields 2026-07-28 requires without
	// re-decoding it into a typed Go struct.
	result, err := mcpWrapResultWithCache(raw, method.cacheable, method.ttlMs, method.cacheScope)
	if err != nil {
		slog.Error("MCP database method: failed to unmarshal result",
			"method", req.Method, "func", method.sqlFunc, "error", err)
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

// mcpToolResult builds a tools/call result envelope. It is a thin wrapper
// around mcpWrapResult that falls back to an unwrapped envelope on the
// (practically unreachable) marshal error, since content only ever holds
// strings and bools.
func mcpToolResult(content []mcpContent, isError bool) map[string]interface{} {
	result, err := mcpWrapResult(mcpToolsCallResult{Content: content, IsError: isError})
	if err != nil {
		return map[string]interface{}{
			"resultType": mcpResultTypeComplete,
			"content":    content,
			"isError":    isError,
		}
	}
	return result
}

// mcpWrapResult wraps a non-cacheable result with the fields the MCP
// specification requires on every result: resultType and _meta.serverInfo.
func mcpWrapResult(result interface{}) (map[string]interface{}, error) {
	return mcpWrapResultWithCache(result, false, 0, "")
}

// mcpWrapCacheableResult wraps a CacheableResult (tools/list, prompts/list,
// resources/list, resources/read, server/discover) with resultType,
// _meta.serverInfo, and the ttlMs/cacheScope freshness hints.
func mcpWrapCacheableResult(result interface{}, ttlMs int64, cacheScope string) (map[string]interface{}, error) {
	return mcpWrapResultWithCache(result, true, ttlMs, cacheScope)
}

// mcpWrapResultWithCache marshals result to a JSON object and adds the
// protocol-mandated fields. result may be a Go struct or a json.RawMessage
// (e.g. a value already shaped by a SQL function) — both marshal cleanly.
func mcpWrapResultWithCache(result interface{}, cacheable bool, ttlMs int64, cacheScope string) (map[string]interface{}, error) {
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		obj = map[string]interface{}{}
	}
	obj["resultType"] = mcpResultTypeComplete
	if cacheable {
		obj["ttlMs"] = ttlMs
		obj["cacheScope"] = cacheScope
	}
	obj["_meta"] = map[string]interface{}{
		mcpMetaServerInfo: mcpImplementation{Name: mcpServerName, Version: version.Version},
	}
	return obj, nil
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
