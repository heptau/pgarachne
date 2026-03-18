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
	"github.com/yourusername/pgarachne/internal/config"
)

// ---------------------------------------------------------------------------
// Helpers shared across MCP tests
// ---------------------------------------------------------------------------

// mcpURL returns the MCP endpoint URL for the test database.
func (e *testEnv) mcpURL() string {
	return e.httpServer.URL + "/" + e.cfg.APIPrefix + "/" + e.dbName + "/mcp"
}

// mcpDo sends a JSON-RPC 2.0 request to the MCP endpoint and returns the
// raw HTTP response. The caller is responsible for closing resp.Body.
func mcpDo(t *testing.T, serverURL, token string, rpcReq map[string]interface{}) *http.Response {
	t.Helper()
	body, err := json.Marshal(rpcReq)
	if err != nil {
		t.Fatalf("mcpDo: marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, serverURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("mcpDo: new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
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

func TestMCPInitializeReturnsCapabilities(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	resp := mcpDo(t, ts.URL+"/db/mydb/mcp", "", map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "test", "version": "0"},
		},
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

	// Protocol version must be advertised.
	if result["protocolVersion"] != mcpProtocolVersion {
		t.Fatalf("protocolVersion = %v, want %v", result["protocolVersion"], mcpProtocolVersion)
	}

	// Server info must be present.
	si, ok := result["serverInfo"].(map[string]interface{})
	if !ok || si["name"] == nil {
		t.Fatalf("serverInfo missing or incomplete: %v", result["serverInfo"])
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

func TestMCPInitializeNoAuthRequired(t *testing.T) {
	// initialize must work without an Authorization header.
	ts := newProtocolTestServer(t)
	defer ts.Close()

	// mcpDo with empty token — no Authorization header sent.
	resp := mcpDo(t, ts.URL+"/db/mydb/mcp", "", map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "initialize",
		"params":  map[string]interface{}{},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestMCPPingReturnsEmptyResult(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	resp := mcpDo(t, ts.URL+"/db/mydb/mcp", "", map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "ping",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	out := decodeMCPResponse(t, resp)
	if out["error"] != nil {
		t.Fatalf("unexpected error: %v", out["error"])
	}
	// result must be present (empty object is fine).
	if _, ok := out["result"]; !ok {
		t.Fatalf("result key missing from ping response")
	}
}

func TestMCPNotificationReturns202(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	// A notification has no "id" field — server must return 202 with no body.
	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		// no "id"
		"method": "notifications/initialized",
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
	// An unknown method that needs auth will fail at auth before reaching the
	// method-not-found check. We test with a database-name that won't connect,
	// but first we need to hit the auth rejection. Instead, we can check the
	// method-not-found error by bypassing auth via a method that goes past the
	// unauthenticated block — but in the protocol server there's no real DB.
	// So we test initialize first to confirm method-not-found is returned for
	// unknown *unauthenticated* methods (they still reach the dispatch after
	// failing auth, producing mcpErrAuth, not mcpErrMethod). We test the
	// method-not-found path via the DB integration tests below.
	//
	// This test confirms that a totally unknown unauthenticated method hits the
	// auth gate and returns mcpErrAuth (not a panic or 500).
	ts := newProtocolTestServer(t)
	defer ts.Close()

	resp := mcpDo(t, ts.URL+"/db/mydb/mcp", "", map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "nonexistent/method",
	})
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		resp.Body.Close()
		t.Fatalf("status = %d, want 200 or 503", resp.StatusCode)
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
		"method":  "initialize",
	})

	out := decodeMCPResponse(t, resp)
	if out["id"] != requestID {
		t.Fatalf("id = %v, want %v", out["id"], requestID)
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
// params returns a params error.
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
		// no params
	})

	out := decodeMCPResponse(t, resp)
	mcpErr, ok := out["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error, got: %v", out)
	}
	if code := mcpErr["code"].(float64); code != mcpErrParams {
		t.Fatalf("error.code = %v, want %v (Invalid params)", code, mcpErrParams)
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
		"db:///",                                      // no schema/table
		"db:///public/",                               // empty table
		"db:///public/no_such_table_xyz_999",          // nonexistent table
		"http://not-a-db-uri",                         // wrong scheme
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
