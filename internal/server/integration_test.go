package server

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/yourusername/pgarachne/internal/config"
)

type testEnv struct {
	cfg        *config.Config
	dbName     string
	testUser   string
	testPass   string
	tokenUser  string
	httpServer *httptest.Server
}

func requireTestEnv(t *testing.T) *testEnv {
	t.Helper()

	if os.Getenv("PGARACHNE_TEST_DB") != "1" {
		t.Skip("set PGARACHNE_TEST_DB=1 to run database integration tests")
	}

	dbName := getenvDefault("TEST_DB_NAME", "pgarachne_test")
	dbHost := getenvDefault("DB_HOST", "localhost")
	dbPortStr := getenvDefault("DB_PORT", "5432")
	dbUser := getenvDefault("DB_USER", "pgarachne")
	jwtSecret := getenvDefault("JWT_SECRET", "test_secret")

	port, err := strconv.Atoi(dbPortStr)
	if err != nil {
		t.Fatalf("invalid DB_PORT: %v", err)
	}

	cfg := &config.Config{
		DBHost:         dbHost,
		DBPort:         port,
		DBUser:         dbUser,
		DBSSLMode:      getenvDefault("DB_SSLMODE", "disable"),
		DBSSLRootCert:  os.Getenv("DB_SSLROOTCERT"),
		DBSSLCert:      os.Getenv("DB_SSLCERT"),
		DBSSLKey:       os.Getenv("DB_SSLKEY"),
		HTTPPort:       "0",
		JWTSecret:      jwtSecret,
		JWTExpiryHours: 1,
		AllowedOrigins: []string{"*"},
		LogLevel:       "ERROR",
		LogOutput:      "stdout",
		LoginRateLimit: getenvIntDefault("LOGIN_RATE_LIMIT", 5),
		LoginRateWindow: func() time.Duration {
			if val := os.Getenv("LOGIN_RATE_WINDOW"); val != "" {
				if d, err := time.ParseDuration(val); err == nil {
					return d
				}
			}
			return time.Minute
		}(),
		MaxRequestBytes: getenvInt64Default("MAX_REQUEST_BYTES", 2*1024*1024),
		SSEMaxChannels:  getenvIntDefault("SSE_MAX_CHANNELS", 8),
		SSEHeartbeat: func() time.Duration {
			if val := os.Getenv("SSE_HEARTBEAT"); val != "" {
				if d, err := time.ParseDuration(val); err == nil {
					return d
				}
			}
			return 20 * time.Second
		}(),
		SSEIdleTimeout: func() time.Duration {
			if val := os.Getenv("SSE_IDLE_TIMEOUT"); val != "" {
				if d, err := time.ParseDuration(val); err == nil {
					return d
				}
			}
			return 90 * time.Second
		}(),
		SSEMaxClients:   getenvIntDefault("SSE_MAX_CLIENTS", 1000),
		SSEClientBuffer: getenvIntDefault("SSE_CLIENT_BUFFER", 64),
		SSESendTimeout: func() time.Duration {
			if val := os.Getenv("SSE_SEND_TIMEOUT"); val != "" {
				if d, err := time.ParseDuration(val); err == nil {
					return d
				}
			}
			return 2 * time.Second
		}(),
	}

	gin.SetMode(gin.TestMode)
	srv := New(cfg)
	ts := httptest.NewServer(srv.buildRouter())

	return &testEnv{
		cfg:        cfg,
		dbName:     dbName,
		testUser:   getenvDefault("TEST_USER", "pgarachne_test_user"),
		testPass:   getenvDefault("TEST_PASSWORD", "pgarachne_test_password"),
		tokenUser:  dbUser,
		httpServer: ts,
	}
}

func (e *testEnv) close() {
	if e.httpServer != nil {
		e.httpServer.Close()
	}
}

func TestLoginAndJWTFlow(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	callPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "api.hello_world",
		"params":  map[string]interface{}{},
		"id":      1,
	}
	callBody, _ := json.Marshal(callPayload)
	callReq, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName, bytes.NewReader(callBody))
	if err != nil {
		t.Fatalf("new call request: %v", err)
	}
	callReq.Header.Set("Content-Type", "application/json")
	callReq.Header.Set("Authorization", "Bearer "+token)

	callResp, err := http.DefaultClient.Do(callReq)
	if err != nil {
		t.Fatalf("call request failed: %v", err)
	}
	defer callResp.Body.Close()

	if callResp.StatusCode != http.StatusOK {
		t.Fatalf("call status = %d, want %d", callResp.StatusCode, http.StatusOK)
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(callResp.Body).Decode(&rpcResp); err != nil {
		t.Fatalf("decode call response: %v", err)
	}
	if rpcResp.Error != nil {
		t.Fatalf("json-rpc error: %s", rpcResp.Error.Message)
	}
	if string(bytes.TrimSpace(rpcResp.Result)) != `"Hello World"` {
		t.Fatalf("unexpected result: %s", rpcResp.Result)
	}
}

func TestAPITokenFlow(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := createAPIToken(env)
	if err != nil {
		t.Fatalf("create api token: %v", err)
	}

	callPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "api.hello_world",
		"params":  map[string]interface{}{},
		"id":      2,
	}
	callBody, _ := json.Marshal(callPayload)
	callReq, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName, bytes.NewReader(callBody))
	if err != nil {
		t.Fatalf("new call request: %v", err)
	}
	callReq.Header.Set("Content-Type", "application/json")
	callReq.Header.Set("Authorization", "Bearer "+token)

	callResp, err := http.DefaultClient.Do(callReq)
	if err != nil {
		t.Fatalf("call request failed: %v", err)
	}
	defer callResp.Body.Close()

	if callResp.StatusCode != http.StatusOK {
		t.Fatalf("call status = %d, want %d", callResp.StatusCode, http.StatusOK)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	_, err := loginAndGetToken(env, env.testUser, "wrong_password")
	if err == nil {
		t.Fatalf("expected login failure with invalid credentials")
	}
}

func TestInvalidFunctionName(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	callPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "api.hello-world",
		"params":  map[string]interface{}{},
		"id":      3,
	}
	callBody, _ := json.Marshal(callPayload)
	callReq, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName, bytes.NewReader(callBody))
	if err != nil {
		t.Fatalf("new call request: %v", err)
	}
	callReq.Header.Set("Content-Type", "application/json")
	callReq.Header.Set("Authorization", "Bearer "+token)

	callResp, err := http.DefaultClient.Do(callReq)
	if err != nil {
		t.Fatalf("call request failed: %v", err)
	}
	defer callResp.Body.Close()

	if callResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("call status = %d, want %d", callResp.StatusCode, http.StatusBadRequest)
	}
}

func TestMissingMethod(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	callPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"params":  map[string]interface{}{},
		"id":      5,
	}
	callBody, _ := json.Marshal(callPayload)
	callReq, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName, bytes.NewReader(callBody))
	if err != nil {
		t.Fatalf("new call request: %v", err)
	}
	callReq.Header.Set("Content-Type", "application/json")
	callReq.Header.Set("Authorization", "Bearer "+token)

	callResp, err := http.DefaultClient.Do(callReq)
	if err != nil {
		t.Fatalf("call request failed: %v", err)
	}
	defer callResp.Body.Close()

	if callResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("call status = %d, want %d", callResp.StatusCode, http.StatusBadRequest)
	}
}

func TestLoginRateLimit(t *testing.T) {
	t.Setenv("LOGIN_RATE_LIMIT", "2")
	t.Setenv("LOGIN_RATE_WINDOW", "1m")

	env := requireTestEnv(t)
	defer env.close()

	status := loginAndGetStatus(env, env.testUser, "wrong_password")
	if status != http.StatusUnauthorized {
		t.Fatalf("first login status = %d, want %d", status, http.StatusUnauthorized)
	}

	status = loginAndGetStatus(env, env.testUser, "wrong_password")
	if status != http.StatusUnauthorized {
		t.Fatalf("second login status = %d, want %d", status, http.StatusUnauthorized)
	}

	status = loginAndGetStatus(env, env.testUser, "wrong_password")
	if status != http.StatusTooManyRequests {
		t.Fatalf("third login status = %d, want %d", status, http.StatusTooManyRequests)
	}
}

func TestMaxRequestBodySize(t *testing.T) {
	t.Setenv("MAX_REQUEST_BYTES", "128")

	env := requireTestEnv(t)
	defer env.close()

	// Create an oversized JSON-RPC payload
	oversized := bytes.Repeat([]byte("a"), 256)
	body := append([]byte(`{"jsonrpc":"2.0","id":1,"method":"login","params":{"login":"x","password":"`), oversized...)
	body = append(body, []byte(`"}}`)...)

	req, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}
}

func TestSSERequiresChannels(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, env.httpServer.URL+"/sse/"+env.dbName, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSSEMaxChannelsLimit(t *testing.T) {
	t.Setenv("SSE_MAX_CHANNELS", "1")

	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, env.httpServer.URL+"/sse/"+env.dbName+"?channels=one,two", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSSEReceivesNotify(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	resp, reader := openSSE(t, env, token, "sse_test_channel")
	defer resp.Body.Close()

	connStr := buildConnStr(env.cfg, env.dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec("NOTIFY sse_test_channel, '{\"id\":123}'"); err != nil {
		t.Fatalf("notify: %v", err)
	}

	payload := readSSEData(t, reader, 3*time.Second)
	if payload["channel"] != "sse_test_channel" {
		t.Fatalf("channel = %v, want %v", payload["channel"], "sse_test_channel")
	}
	data, ok := payload["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data type = %T, want object", payload["data"])
	}
	if data["id"] != float64(123) {
		t.Fatalf("data.id = %v, want 123", data["id"])
	}
}

func TestSSEQuotedChannel(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	channelName := `Quoted Channel`
	resp, reader := openSSE(t, env, token, `"`+channelName+`"`)
	defer resp.Body.Close()

	connStr := buildConnStr(env.cfg, env.dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(fmt.Sprintf(`NOTIFY "%s", '{"ok":true}'`, channelName)); err != nil {
		t.Fatalf("notify: %v", err)
	}

	payload := readSSEData(t, reader, 3*time.Second)
	if payload["channel"] != channelName {
		t.Fatalf("channel = %v, want %v", payload["channel"], channelName)
	}
}

func TestSSEMaxClientsLimit(t *testing.T) {
	t.Setenv("SSE_MAX_CLIENTS", "1")

	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	resp1, _ := openSSE(t, env, token, "limit_channel")
	defer resp1.Body.Close()

	req, err := http.NewRequest(http.MethodGet, env.httpServer.URL+"/sse/"+env.dbName+"?channels=limit_channel", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", resp2.StatusCode, http.StatusTooManyRequests)
	}
}

func TestCapabilitiesRespectsExecutePrivilege(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	functionName := fmt.Sprintf("api.private_fn_%d", time.Now().UnixNano())
	if err := createPrivateFunction(env, functionName); err != nil {
		t.Fatalf("create private function: %v", err)
	}

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	methods, err := fetchCapabilities(env, token)
	if err != nil {
		t.Fatalf("capabilities fetch failed: %v", err)
	}
	if containsMethod(methods, functionName) {
		t.Fatalf("expected %s to be hidden without EXECUTE", functionName)
	}

	if err := grantExecute(env, functionName); err != nil {
		t.Fatalf("grant execute: %v", err)
	}

	methods, err = fetchCapabilities(env, token)
	if err != nil {
		t.Fatalf("capabilities fetch failed after grant: %v", err)
	}
	if !containsMethod(methods, functionName) {
		t.Fatalf("expected %s to be visible after EXECUTE grant", functionName)
	}
}

func createAPIToken(env *testEnv) (string, error) {
	connStr := buildConnStr(env.cfg, env.dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return "", err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var token string
	if err := db.QueryRowContext(ctx, `SELECT pgarachne.add_api_token($1, $2, NULL)`, "test token", env.tokenUser).Scan(&token); err != nil {
		return "", err
	}
	return token, nil
}

func createPrivateFunction(env *testEnv, functionName string) error {
	connStr := buildConnStr(env.cfg, env.dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = db.ExecContext(ctx, fmt.Sprintf(`
CREATE OR REPLACE FUNCTION %s(payload jsonb)
RETURNS json
LANGUAGE sql
AS $$
  SELECT '"private"'::json;
$$;
REVOKE EXECUTE ON FUNCTION %s(jsonb) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION %s(jsonb) FROM %s;
`, functionName, functionName, functionName, env.testUser))
	return err
}

func grantExecute(env *testEnv, functionName string) error {
	connStr := buildConnStr(env.cfg, env.dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = db.ExecContext(ctx, fmt.Sprintf("GRANT EXECUTE ON FUNCTION %s(jsonb) TO %s;", functionName, env.testUser))
	return err
}

func fetchCapabilities(env *testEnv, token string) ([]string, error) {
	callPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "capabilities",
		"params":  map[string]interface{}{},
		"id":      4,
	}
	callBody, _ := json.Marshal(callPayload)
	callReq, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName, bytes.NewReader(callBody))
	if err != nil {
		return nil, err
	}
	callReq.Header.Set("Content-Type", "application/json")
	callReq.Header.Set("Authorization", "Bearer "+token)

	callResp, err := http.DefaultClient.Do(callReq)
	if err != nil {
		return nil, err
	}
	defer callResp.Body.Close()

	if callResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("capabilities status = %d", callResp.StatusCode)
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(callResp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("json-rpc error: %s", rpcResp.Error.Message)
	}

	var methods []struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(rpcResp.Result, &methods); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(methods))
	for _, m := range methods {
		out = append(out, m.Method)
	}
	return out, nil
}

func containsMethod(methods []string, target string) bool {
	for _, m := range methods {
		if m == target {
			return true
		}
	}
	return false
}

func loginAndGetToken(env *testEnv, login, password string) (string, error) {
	loginPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "login",
		"params": map[string]string{
			"login":    login,
			"password": password,
		},
		"id": 1,
	}
	body, _ := json.Marshal(loginPayload)

	req, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login status = %d", resp.StatusCode)
	}

	var loginResp struct {
		Result struct {
			Token string `json:"token"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", err
	}
	if loginResp.Error != nil {
		return "", fmt.Errorf("json-rpc error: %s", loginResp.Error.Message)
	}
	if loginResp.Result.Token == "" {
		return "", fmt.Errorf("empty token")
	}
	return loginResp.Result.Token, nil
}

func loginAndGetStatus(env *testEnv, login, password string) int {
	loginPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "login",
		"params": map[string]string{
			"login":    login,
			"password": password,
		},
		"id": 1,
	}
	body, _ := json.Marshal(loginPayload)

	req, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName, bytes.NewReader(body))
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func openSSE(t *testing.T, env *testEnv, token, channels string) (*http.Response, *bufio.Reader) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, env.httpServer.URL+"/sse/"+env.dbName+"?channels="+url.QueryEscape(channels), nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	return resp, bufio.NewReader(resp.Body)
}

func readSSEData(t *testing.T, reader *bufio.Reader, timeout time.Duration) map[string]interface{} {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for SSE data")
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read sse: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			var out map[string]interface{}
			if err := json.Unmarshal([]byte(payload), &out); err != nil {
				t.Fatalf("invalid sse json: %v", err)
			}
			return out
		}
	}
}

func buildConnStr(cfg *config.Config, dbName string) string {
	return "host=" + cfg.DBHost +
		" port=" + strconv.Itoa(cfg.DBPort) +
		" user=" + cfg.DBUser +
		" dbname=" + dbName +
		" " + cfg.DBSSLParams()
}

func getenvDefault(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

func getenvIntDefault(key string, def int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return def
}

func getenvInt64Default(key string, def int64) int64 {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
	}
	return def
}
