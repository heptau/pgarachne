package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/heptau/pgarachne/internal/config"
)

// ---------------------------------------------------------------------------
// Helpers shared across MCP tests
// ---------------------------------------------------------------------------

// mcpURL returns the MCP endpoint URL for the test database.
func (e *testEnv) mcpURL() string {
	return e.httpServer.URL + "/" + e.cfg.APIPrefix + "/" + e.dbName + "/mcp"
}

// mcpInjectMeta fills in the params._meta fields protocol version 2026-07-28
// requires on every non-notification request (protocolVersion and
// clientCapabilities), unless the test already set them. This keeps
// individual test bodies focused on the method-specific params they care
// about instead of repeating transport boilerplate everywhere.
func mcpInjectMeta(rpcReq map[string]interface{}) {
	method, ok := rpcReq["method"].(string)
	if !ok || strings.HasPrefix(method, "notifications/") {
		return
	}
	params, ok := rpcReq["params"].(map[string]interface{})
	if !ok {
		params = map[string]interface{}{}
		rpcReq["params"] = params
	}
	meta, ok := params["_meta"].(map[string]interface{})
	if !ok {
		meta = map[string]interface{}{}
		params["_meta"] = meta
	}
	if _, ok := meta[mcpMetaProtocolVersion]; !ok {
		meta[mcpMetaProtocolVersion] = mcpProtocolVersion
	}
	if _, ok := meta[mcpMetaClientCapabilities]; !ok {
		meta[mcpMetaClientCapabilities] = map[string]interface{}{}
	}
}

// mcpNameHeaderValue returns the value the Mcp-Name header must carry for
// name/uri-addressed methods, or "" if the method does not require one.
func mcpNameHeaderValue(rpcReq map[string]interface{}) string {
	method, _ := rpcReq["method"].(string)
	if !requiresNameHeader(method) {
		return ""
	}
	params, ok := rpcReq["params"].(map[string]interface{})
	if !ok {
		return ""
	}
	key := "name"
	if method == "resources/read" {
		key = "uri"
	}
	v, _ := params[key].(string)
	return v
}

// mcpDo sends a JSON-RPC 2.0 request to the MCP endpoint and returns the
// raw HTTP response. The caller is responsible for closing resp.Body.
//
// It fills in the params._meta protocol fields and the standard
// MCP-Protocol-Version / Mcp-Method / Mcp-Name headers a compliant
// 2026-07-28 client must send, so individual tests only need to supply the
// method-specific parts of the request. Tests that specifically exercise
// malformed envelopes or missing headers build the request by hand instead.
func mcpDo(t *testing.T, serverURL, token string, rpcReq map[string]interface{}) *http.Response {
	t.Helper()
	mcpInjectMeta(rpcReq)
	body, err := json.Marshal(rpcReq)
	if err != nil {
		t.Fatalf("mcpDo: marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, serverURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("mcpDo: new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if method, ok := rpcReq["method"].(string); ok && !strings.HasPrefix(method, "notifications/") {
		req.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
		req.Header.Set("Mcp-Method", method)
		if name := mcpNameHeaderValue(rpcReq); name != "" {
			req.Header.Set("Mcp-Name", name)
		}
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("mcpDo: do: %v", err)
	}
	return resp
}

// decodeMCPResponse decodes the HTTP response body into a generic map so tests
// can inspect both result and error fields without strong typing.
func decodeMCPResponse(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()
	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decodeMCPResponse: %v", err)
	}
	return out
}

// newProtocolTestServer creates a minimal *httptest.Server that only needs the
// MCP protocol-level handlers (initialize, ping, notifications/*). It does NOT
// require a real database — calls that need DB will fail with a connection
// error, but the protocol-level paths never touch the DB.
func newProtocolTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		DBHost:          "127.0.0.1",
		DBPort:          5432,
		DBUser:          "pgarachne",
		JWTSecret:       "test_secret",
		JWTExpiryHours:  1,
		AllowedOrigins:  []string{"*"},
		MaxRequestBytes: 2 * 1024 * 1024,
		APIPrefix:       "db",
		MetricsEnabled:  false,
		SSEMaxChannels:  8,
		SSEMaxClients:   1000,
		SSEClientBuffer: 64,
		SSEHeartbeat:    20 * time.Second,
		SSEIdleTimeout:  90 * time.Second,
		SSESendTimeout:  2 * time.Second,
	}
	srv := New(cfg)
	return httptest.NewServer(srv.buildRouter())
}

// ---------------------------------------------------------------------------
// Protocol-level tests — no database required
// These tests exercise the MCP transport and envelope handling. They do NOT
// set PGARACHNE_TEST_DB=1 and therefore do not skip.
// ---------------------------------------------------------------------------

func TestMCPServerDiscoverReturnsCapabilities(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	resp := mcpDo(t, ts.URL+"/db/mydb/mcp", "", map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "server/discover",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)

	if out["error"] != nil {
		t.Fatalf("unexpected error: %v", out["error"])
	}

	result, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result missing or not an object: %v", out["result"])
	}

	if result["resultType"] != "complete" {
		t.Fatalf("resultType = %v, want complete", result["resultType"])
	}

	// Supported protocol versions must be advertised.
	versions, ok := result["supportedVersions"].([]interface{})
	if !ok || len(versions) == 0 || versions[0] != mcpProtocolVersion {
		t.Fatalf("supportedVersions = %v, want [%v]", result["supportedVersions"], mcpProtocolVersion)
	}

	// Server info must be present under _meta.
	meta, ok := result["_meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("_meta missing: %v", result["_meta"])
	}
	si, ok := meta[mcpMetaServerInfo].(map[string]interface{})
	if !ok || si["name"] == nil {
		t.Fatalf("serverInfo missing or incomplete: %v", meta[mcpMetaServerInfo])
	}

	// This is a CacheableResult.
	if result["ttlMs"] == nil || result["cacheScope"] == nil {
		t.Fatalf("cache hints missing: ttlMs=%v cacheScope=%v", result["ttlMs"], result["cacheScope"])
	}

	// Capabilities must advertise tools, resources, and prompts.
	caps, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("capabilities missing: %v", result["capabilities"])
	}
	for _, key := range []string{"tools", "resources", "prompts"} {
		if caps[key] == nil {
			t.Fatalf("capabilities.%s missing", key)
		}
	}
}

func TestMCPServerDiscoverNoAuthRequired(t *testing.T) {
	// server/discover must work without an Authorization header.
	ts := newProtocolTestServer(t)
	defer ts.Close()

	// mcpDo with empty token — no Authorization header sent.
	resp := mcpDo(t, ts.URL+"/db/mydb/mcp", "", map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "server/discover",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestMCPNotificationReturns202(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	// A notification has a method name in the notifications/ namespace.
	// The server must return 202 with no body.
	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/db/mydb/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}

	// Body must be empty.
	bodyBytes, _ := io.ReadAll(resp.Body)
	if len(bytes.TrimSpace(bodyBytes)) != 0 {
		t.Fatalf("expected empty body for notification, got: %s", bodyBytes)
	}
}

func TestMCPRequestWithNullIDGetsResponse(t *testing.T) {
	// A request with id: null is still a request per JSON-RPC 2.0: the
	// client expects a JSON-RPC response (whose id is also null). The server
	// must NOT treat it as a notification and return 202.
	ts := newProtocolTestServer(t)
	defer ts.Close()

	resp := mcpDo(t, ts.URL+"/db/mydb/mcp", "", map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      nil,
		"method":  "server/discover",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	if out["error"] != nil {
		t.Fatalf("unexpected error: %v", out["error"])
	}
	// id must round-trip as null (not absent).
	if _, present := out["id"]; !present {
		t.Fatalf("id key missing from response")
	}
	if out["id"] != nil {
		t.Fatalf("id = %v, want nil", out["id"])
	}
}

func TestMCPInvalidJSONRPCVersion(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	resp := mcpDo(t, ts.URL+"/db/mydb/mcp", "", map[string]interface{}{
		"jsonrpc": "1.0",
		"id":      5,
		"method":  "initialize",
		"params":  map[string]interface{}{},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	mcpErr, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error field, got: %v", out)
	}
	if code := mcpErr["code"].(float64); code != mcpErrInvalid {
		t.Fatalf("error.code = %v, want %v (Invalid Request)", code, mcpErrInvalid)
	}
}

func TestMCPParseError(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/db/mydb/mcp",
		bytes.NewReader([]byte(`{not valid json`)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	mcpErr, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error field, got: %v", out)
	}
	if code := mcpErr["code"].(float64); code != mcpErrParse {
		t.Fatalf("error.code = %v, want %v (Parse error)", code, mcpErrParse)
	}
}

func TestMCPUnknownMethodReturnsMethodNotFound(t *testing.T) {
	// An unknown method still needs auth: it isn't "server/discover", so it
	// falls through to the auth gate before method dispatch ever runs. With no
	// real database behind this protocol-only test server, that gate fails
	// either while connecting (503) or, if a connection is somehow obtained,
	// on the missing Authorization header (401) — either way this confirms an
	// unrecognized method doesn't panic or 500 before reaching it. The actual
	// method-not-found path (mcpErrMethod, HTTP 404) is exercised against a
	// real authenticated connection in TestMCPUnknownMethodAfterAuth.
	ts := newProtocolTestServer(t)
	defer ts.Close()

	resp := mcpDo(t, ts.URL+"/db/mydb/mcp", "", map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "nonexistent/method",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 503 or 401", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	if out["error"] == nil {
		t.Fatalf("expected error for unknown method, got result: %v", out["result"])
	}
}

func TestMCPIDPreservedInErrorResponse(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	const requestID = float64(42)
	resp := mcpDo(t, ts.URL+"/db/mydb/mcp", "", map[string]interface{}{
		"jsonrpc": "1.0", // invalid version — will error
		"id":      requestID,
		"method":  "server/discover",
	})

	out := decodeMCPResponse(t, resp)
	if out["id"] != requestID {
		t.Fatalf("id = %v, want %v", out["id"], requestID)
	}
}

// ---------------------------------------------------------------------------
// Transport validation tests (protocol version 2026-07-28) — no database
// required. These exercise the per-request _meta and Mcp-* header checks
// introduced by the stateless, handshake-free protocol revision.
// ---------------------------------------------------------------------------

func TestMCPMissingMetaReturnsInvalidParams(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	// Bypass mcpDo's auto-injection to send a request with no params._meta at all.
	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "server/discover",
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/db/mydb/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	req.Header.Set("Mcp-Method", "server/discover")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	mcpErr, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error, got: %v", out)
	}
	if code := mcpErr["code"].(float64); code != mcpErrParams {
		t.Fatalf("error.code = %v, want %v (Invalid params)", code, mcpErrParams)
	}
}

func TestMCPMissingProtocolVersionHeaderReturnsHeaderMismatch(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "server/discover",
		"params": map[string]interface{}{
			"_meta": map[string]interface{}{
				mcpMetaProtocolVersion:    mcpProtocolVersion,
				mcpMetaClientCapabilities: map[string]interface{}{},
			},
		},
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/db/mydb/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Method", "server/discover")
	// MCP-Protocol-Version header intentionally omitted.

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	mcpErr, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error, got: %v", out)
	}
	if code := mcpErr["code"].(float64); code != mcpErrHeaderMismatch {
		t.Fatalf("error.code = %v, want %v (HeaderMismatch)", code, mcpErrHeaderMismatch)
	}
}

func TestMCPProtocolVersionHeaderBodyMismatchReturnsHeaderMismatch(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "server/discover",
		"params": map[string]interface{}{
			"_meta": map[string]interface{}{
				mcpMetaProtocolVersion:    mcpProtocolVersion,
				mcpMetaClientCapabilities: map[string]interface{}{},
			},
		},
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/db/mydb/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25") // disagrees with the body
	req.Header.Set("Mcp-Method", "server/discover")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	mcpErr, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error, got: %v", out)
	}
	if code := mcpErr["code"].(float64); code != mcpErrHeaderMismatch {
		t.Fatalf("error.code = %v, want %v (HeaderMismatch)", code, mcpErrHeaderMismatch)
	}
}

func TestMCPUnsupportedProtocolVersionReturnsError(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "server/discover",
		"params": map[string]interface{}{
			"_meta": map[string]interface{}{
				mcpMetaProtocolVersion:    "1900-01-01",
				mcpMetaClientCapabilities: map[string]interface{}{},
			},
		},
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/db/mydb/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", "1900-01-01")
	req.Header.Set("Mcp-Method", "server/discover")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	mcpErr, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error, got: %v", out)
	}
	if code := mcpErr["code"].(float64); code != mcpErrUnsupportedVersion {
		t.Fatalf("error.code = %v, want %v (UnsupportedProtocolVersion)", code, mcpErrUnsupportedVersion)
	}
	data, ok := mcpErr["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error.data with supported versions, got: %v", mcpErr["data"])
	}
	if data["requested"] != "1900-01-01" {
		t.Fatalf("data.requested = %v, want 1900-01-01", data["requested"])
	}
}

func TestMCPMissingMcpMethodHeaderReturnsHeaderMismatch(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "server/discover",
		"params": map[string]interface{}{
			"_meta": map[string]interface{}{
				mcpMetaProtocolVersion:    mcpProtocolVersion,
				mcpMetaClientCapabilities: map[string]interface{}{},
			},
		},
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/db/mydb/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MCP-Protocol-Version", mcpProtocolVersion)
	// Mcp-Method header intentionally omitted.

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	mcpErr, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error, got: %v", out)
	}
	if code := mcpErr["code"].(float64); code != mcpErrHeaderMismatch {
		t.Fatalf("error.code = %v, want %v (HeaderMismatch)", code, mcpErrHeaderMismatch)
	}
}

func TestMCPGetAndDeleteReturn405(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		req, _ := http.NewRequest(method, ts.URL+"/db/mydb/mcp", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s request failed: %v", method, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("%s status = %d, want 405", method, resp.StatusCode)
		}
	}
}

// ---------------------------------------------------------------------------
// Integration tests — require PGARACHNE_TEST_DB=1
// ---------------------------------------------------------------------------

// TestMCPToolsListReturnsTools verifies that tools/list returns at least the
// hello_world function that the test schema exposes.
func TestMCPToolsListReturnsTools(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	})
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	if out["error"] != nil {
		t.Fatalf("unexpected error: %v", out["error"])
	}

	result, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result not an object: %v", out["result"])
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("result.tools not an array: %v", result["tools"])
	}
	if len(tools) == 0 {
		t.Fatalf("expected at least one tool, got empty list")
	}

	// Each tool must have name, description, and inputSchema.
	for i, raw := range tools {
		tool, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("tool[%d] not an object", i)
		}
		if tool["name"] == nil || tool["name"] == "" {
			t.Fatalf("tool[%d].name missing", i)
		}
		if tool["inputSchema"] == nil {
			t.Fatalf("tool[%d].inputSchema missing", i)
		}
	}
}

// TestMCPToolsListRespectsPrivileges verifies that tools/list only shows
// functions the authenticated user has EXECUTE on — same as capabilities.
func TestMCPToolsListRespectsPrivileges(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	functionName := fmt.Sprintf("api.mcp_private_%d", time.Now().UnixNano())
	if err := createPrivateFunction(env, functionName); err != nil {
		t.Fatalf("create private function: %v", err)
	}

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	tools := mcpFetchToolNames(t, env, token)
	for _, name := range tools {
		if name == functionName {
			t.Fatalf("private function %s visible before EXECUTE grant", functionName)
		}
	}

	if err := grantExecute(env, functionName); err != nil {
		t.Fatalf("grant execute: %v", err)
	}

	tools = mcpFetchToolNames(t, env, token)
	found := false
	for _, name := range tools {
		if name == functionName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("function %s not visible after EXECUTE grant", functionName)
	}
}

// TestMCPToolsCallSuccess verifies that tools/call executes a PostgreSQL
// function and wraps the result in a text content block.
func TestMCPToolsCallSuccess(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "api.hello_world",
			"arguments": map[string]interface{}{},
		},
	})
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	if out["error"] != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", out["error"])
	}

	result, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result not an object: %v", out["result"])
	}

	// isError must be absent or false.
	if isErr, _ := result["isError"].(bool); isErr {
		t.Fatalf("isError = true, expected successful call")
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("result.content empty or missing")
	}

	block, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("content[0] not an object")
	}
	if block["type"] != "text" {
		t.Fatalf("content[0].type = %v, want text", block["type"])
	}
	if !strings.Contains(fmt.Sprint(block["text"]), "Hello World") {
		t.Fatalf("content[0].text does not contain 'Hello World': %v", block["text"])
	}
}

// TestMCPToolsCallNonexistentFunction verifies that calling a function that
// does not exist returns a tool-level error (isError: true), not a JSON-RPC
// protocol error — per MCP specification §tool-result.
func TestMCPToolsCallNonexistentFunction(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "api.no_such_function_xyz",
			"arguments": map[string]interface{}{},
		},
	})

	out := decodeMCPResponse(t, resp)

	// Must NOT be a protocol-level error.
	if out["error"] != nil {
		t.Fatalf("expected tool-level error, got protocol error: %v", out["error"])
	}

	result, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result not an object: %v", out["result"])
	}
	if isErr, _ := result["isError"].(bool); !isErr {
		t.Fatalf("isError = false, expected tool-level error for nonexistent function")
	}
}

// TestMCPToolsCallInvalidToolName verifies that an unsafe tool name (one that
// fails the identifier validation) is rejected at the protocol level.
func TestMCPToolsCallInvalidToolName(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "api.hello-world", // dash is not allowed
			"arguments": map[string]interface{}{},
		},
	})

	out := decodeMCPResponse(t, resp)
	mcpErr, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected protocol error, got: %v", out)
	}
	if code := mcpErr["code"].(float64); code != mcpErrParams {
		t.Fatalf("error.code = %v, want %v (Invalid params)", code, mcpErrParams)
	}
}

// TestMCPToolsCallMissingParamsReturnsError verifies that tools/call without
// a tool name is rejected. Since a name is required to fill the Mcp-Name
// header, this is now caught by header validation (HeaderMismatch) before
// the request ever reaches the tools/call handler's own params check.
func TestMCPToolsCallMissingParamsReturnsError(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		// no params, so no tool name to send in Mcp-Name.
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	mcpErr, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error, got: %v", out)
	}
	if code := mcpErr["code"].(float64); code != mcpErrHeaderMismatch {
		t.Fatalf("error.code = %v, want %v (HeaderMismatch)", code, mcpErrHeaderMismatch)
	}
}

// TestMCPRequiresAuthForToolsList verifies that tools/list rejects requests
// without a valid Authorization header.
func TestMCPRequiresAuthForToolsList(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	resp := mcpDo(t, env.mcpURL(), "" /* no token */, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	})

	out := decodeMCPResponse(t, resp)
	mcpErr, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected auth error, got: %v", out)
	}
	if code := mcpErr["code"].(float64); code != mcpErrAuth {
		t.Fatalf("error.code = %v, want %v (Auth error)", code, mcpErrAuth)
	}
}

// TestMCPRequiresAuthForResourcesList verifies that resources/list rejects
// requests without a valid Authorization header.
func TestMCPRequiresAuthForResourcesList(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	resp := mcpDo(t, env.mcpURL(), "", map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/list",
		"params":  map[string]interface{}{},
	})

	out := decodeMCPResponse(t, resp)
	mcpErr, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected auth error, got: %v", out)
	}
	if code := mcpErr["code"].(float64); code != mcpErrAuth {
		t.Fatalf("error.code = %v, want %v (Auth error)", code, mcpErrAuth)
	}
}

// TestMCPUnknownMethodAfterAuth verifies that a non-existent authenticated
// method returns -32601 Method Not Found.
func TestMCPUnknownMethodAfterAuth(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "totally/unknown",
		"params":  map[string]interface{}{},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	mcpErr, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error, got: %v", out)
	}
	if code := mcpErr["code"].(float64); code != mcpErrMethod {
		t.Fatalf("error.code = %v, want %v (Method Not Found)", code, mcpErrMethod)
	}
}

// TestMCPResourcesListReturnsTables verifies that resources/list returns
// an object with a "resources" array, and each resource has uri, name, and
// mimeType fields.
func TestMCPResourcesListReturnsTables(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/list",
		"params":  map[string]interface{}{},
	})
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	if out["error"] != nil {
		t.Fatalf("unexpected error: %v", out["error"])
	}

	result, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result not an object: %v", out["result"])
	}

	resources, ok := result["resources"].([]interface{})
	if !ok {
		t.Fatalf("result.resources not an array: %v", result["resources"])
	}

	// The test database must have at least the pgarachne internal tables visible
	// to the service user. We just verify the shape is correct.
	for i, raw := range resources {
		res, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("resources[%d] not an object", i)
		}
		uri, _ := res["uri"].(string)
		if !strings.HasPrefix(uri, "db:///") {
			t.Fatalf("resources[%d].uri = %q, want db:/// prefix", i, uri)
		}
		if res["name"] == nil {
			t.Fatalf("resources[%d].name missing", i)
		}
		if res["mimeType"] != "application/json" {
			t.Fatalf("resources[%d].mimeType = %v, want application/json", i, res["mimeType"])
		}
	}
}

// TestMCPResourcesReadReturnsContents verifies that resources/read returns a
// contents array with uri, mimeType, and text fields for a known table.
func TestMCPResourcesReadReturnsContents(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// First list resources to get a valid URI we can read.
	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/list",
		"params":  map[string]interface{}{},
	})
	listOut := decodeMCPResponse(t, resp)
	resources, _ := listOut["result"].(map[string]interface{})["resources"].([]interface{})
	if len(resources) == 0 {
		t.Skip("no resources available to read")
	}

	firstURI := resources[0].(map[string]interface{})["uri"].(string)

	resp = mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "resources/read",
		"params":  map[string]interface{}{"uri": firstURI},
	})
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	if out["error"] != nil {
		t.Fatalf("unexpected error: %v", out["error"])
	}

	result, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result not an object: %v", out["result"])
	}

	contents, ok := result["contents"].([]interface{})
	if !ok || len(contents) == 0 {
		t.Fatalf("result.contents missing or empty: %v", result["contents"])
	}

	block, ok := contents[0].(map[string]interface{})
	if !ok {
		t.Fatalf("contents[0] not an object")
	}
	if block["uri"] != firstURI {
		t.Fatalf("contents[0].uri = %v, want %v", block["uri"], firstURI)
	}
	if block["mimeType"] != "application/json" {
		t.Fatalf("contents[0].mimeType = %v, want application/json", block["mimeType"])
	}
	if block["text"] == nil {
		t.Fatalf("contents[0].text missing")
	}
}

// TestMCPResourcesReadInvalidURIReturnsError verifies that resources/read
// returns a protocol-level error for a malformed or non-existent URI.
func TestMCPResourcesReadInvalidURIReturnsError(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	for _, uri := range []string{
		"db:///",                             // no schema/table
		"db:///public/",                      // empty table
		"db:///public/no_such_table_xyz_999", // nonexistent table
		"http://not-a-db-uri",                // wrong scheme
	} {
		resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "resources/read",
			"params":  map[string]interface{}{"uri": uri},
		})

		out := decodeMCPResponse(t, resp)
		if out["error"] == nil {
			t.Fatalf("uri=%q: expected error, got result: %v", uri, out["result"])
		}
	}
}

// TestMCPResourcesReadMissingURIReturnsError verifies that resources/read
// without a uri param returns a protocol-level error.
func TestMCPResourcesReadMissingURIReturnsError(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/read",
		"params":  map[string]interface{}{}, // no uri
	})

	out := decodeMCPResponse(t, resp)
	if out["error"] == nil {
		t.Fatalf("expected error for missing uri, got result: %v", out["result"])
	}
}

// TestMCPPromptsListReturnsArray verifies that prompts/list returns a
// "prompts" array (possibly empty if no prompts are inserted yet).
func TestMCPPromptsListReturnsArray(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "prompts/list",
		"params":  map[string]interface{}{},
	})
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	if out["error"] != nil {
		t.Fatalf("unexpected error: %v", out["error"])
	}

	result, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result not an object: %v", out["result"])
	}
	if _, ok := result["prompts"].([]interface{}); !ok {
		t.Fatalf("result.prompts not an array: %v", result["prompts"])
	}
}

// TestMCPPromptsGetAndList inserts a prompt via SQL, lists it via prompts/list,
// then retrieves the rendered version via prompts/get with argument substitution.
func TestMCPPromptsGetAndList(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	// Insert a test prompt directly in the DB.
	promptName := fmt.Sprintf("test_prompt_%d", time.Now().UnixNano())
	if err := insertTestPrompt(env, promptName, "Test prompt", "Hello {{name}}!", `[{"name":"name","description":"A name","required":true}]`); err != nil {
		t.Fatalf("insert prompt: %v", err)
	}
	defer deleteTestPrompt(env, promptName) //nolint:errcheck

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// Verify prompts/list shows the new prompt.
	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "prompts/list",
		"params":  map[string]interface{}{},
	})
	listOut := decodeMCPResponse(t, resp)
	if listOut["error"] != nil {
		t.Fatalf("prompts/list error: %v", listOut["error"])
	}
	listResult, ok := listOut["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("prompts/list result not an object: %v", listOut["result"])
	}
	prompts, _ := listResult["prompts"].([]interface{})
	found := false
	for _, raw := range prompts {
		p, _ := raw.(map[string]interface{})
		if p["name"] == promptName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("prompt %s not found in prompts/list", promptName)
	}

	// Verify prompts/get renders {{name}} with the provided argument.
	resp = mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "prompts/get",
		"params": map[string]interface{}{
			"name":      promptName,
			"arguments": map[string]string{"name": "World"},
		},
	})
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	if out["error"] != nil {
		t.Fatalf("unexpected error: %v", out["error"])
	}

	result, ok := out["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result not an object: %v", out["result"])
	}

	messages, ok := result["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		t.Fatalf("result.messages missing or empty: %v", result["messages"])
	}

	msg, _ := messages[0].(map[string]interface{})
	if msg["role"] != "user" {
		t.Fatalf("messages[0].role = %v, want user", msg["role"])
	}

	content, _ := msg["content"].(map[string]interface{})
	text, _ := content["text"].(string)
	if text != "Hello World!" {
		t.Fatalf("rendered text = %q, want %q", text, "Hello World!")
	}
}

// TestMCPPromptsGetMissingPromptReturnsError verifies that prompts/get for a
// non-existent prompt returns a protocol-level error.
func TestMCPPromptsGetMissingPromptReturnsError(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "prompts/get",
		"params":  map[string]interface{}{"name": "no_such_prompt_xyz"},
	})

	out := decodeMCPResponse(t, resp)
	if out["error"] == nil {
		t.Fatalf("expected error for missing prompt, got result: %v", out["result"])
	}
}

// TestMCPPromptsGetMissingNameReturnsError verifies that prompts/get without
// a name param returns a protocol-level error.
func TestMCPPromptsGetMissingNameReturnsError(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "prompts/get",
		"params":  map[string]interface{}{}, // missing name
	})

	out := decodeMCPResponse(t, resp)
	if out["error"] == nil {
		t.Fatalf("expected error for missing name, got result: %v", out["result"])
	}
}

// TestMCPIdempotencyKeyDeduplication verifies that two tools/call requests
// with the same idempotency key: the first succeeds and the second is rejected
// as a duplicate with isError: true.
func TestMCPIdempotencyKeyDeduplication(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	idemKey := fmt.Sprintf("mcp-idem-%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "api.hello_world",
			"arguments": map[string]interface{}{},
			"_meta":     map[string]interface{}{"idempotencyKey": idemKey},
		},
	}

	// First call — must succeed.
	first := decodeMCPResponse(t, mcpDo(t, env.mcpURL(), token, payload))
	if first["error"] != nil {
		t.Fatalf("first call: unexpected protocol error: %v", first["error"])
	}
	if result, ok := first["result"].(map[string]interface{}); ok {
		if isErr, _ := result["isError"].(bool); isErr {
			t.Fatalf("first call: unexpected isError = true")
		}
	}

	// Second call with same key — must be rejected as duplicate.
	second := decodeMCPResponse(t, mcpDo(t, env.mcpURL(), token, payload))
	if second["error"] != nil {
		t.Fatalf("second call: unexpected protocol error: %v", second["error"])
	}
	result, ok := second["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("second call: result not an object: %v", second["result"])
	}
	if isErr, _ := result["isError"].(bool); !isErr {
		t.Fatalf("second call: expected isError = true for duplicate idempotency key")
	}
}

// TestMCPUsesAPITokenAuth verifies that a long-lived API token works with the
// MCP endpoint (same as with JSON-RPC).
func TestMCPUsesAPITokenAuth(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := createAPIToken(env)
	if err != nil {
		t.Fatalf("create api token: %v", err)
	}

	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	})
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	if out["error"] != nil {
		t.Fatalf("unexpected error with API token: %v", out["error"])
	}
}

// ---------------------------------------------------------------------------
// Test helpers specific to MCP
// ---------------------------------------------------------------------------

// mcpFetchToolNames calls tools/list and returns the list of tool names.
func mcpFetchToolNames(t *testing.T, env *testEnv, token string) []string {
	t.Helper()
	resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	})
	out := decodeMCPResponse(t, resp)
	result, _ := out["result"].(map[string]interface{})
	tools, _ := result["tools"].([]interface{})
	names := make([]string, 0, len(tools))
	for _, raw := range tools {
		if tool, ok := raw.(map[string]interface{}); ok {
			if name, ok := tool["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}

// insertTestPrompt inserts a prompt into pgarachne.prompts via the service
// user connection (which has full access to the pgarachne schema).
func insertTestPrompt(env *testEnv, name, description, template, arguments string) error {
	connStr := buildConnStr(env.cfg, env.dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`
		INSERT INTO pgarachne.prompts (name, description, template, arguments)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (name) DO UPDATE
		  SET description = EXCLUDED.description,
		      template    = EXCLUDED.template,
		      arguments   = EXCLUDED.arguments`,
		name, description, template, arguments)
	return err
}

// deleteTestPrompt removes a test prompt from pgarachne.prompts.
func deleteTestPrompt(env *testEnv, name string) error {
	connStr := buildConnStr(env.cfg, env.dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`DELETE FROM pgarachne.prompts WHERE name = $1`, name)
	return err
}
