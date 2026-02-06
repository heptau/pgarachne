package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
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

	// Create an oversized JSON payload
	oversized := bytes.Repeat([]byte("a"), 256)
	body := append([]byte(`{"login":"x","password":"`), oversized...)
	body = append(body, []byte(`"}`)...)

	req, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName+"/login", bytes.NewReader(body))
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
	loginPayload := map[string]string{
		"login":    login,
		"password": password,
	}
	body, _ := json.Marshal(loginPayload)

	req, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName+"/login", bytes.NewReader(body))
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
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", err
	}
	if loginResp.Token == "" {
		return "", fmt.Errorf("empty token")
	}
	return loginResp.Token, nil
}

func loginAndGetStatus(env *testEnv, login, password string) int {
	loginPayload := map[string]string{
		"login":    login,
		"password": password,
	}
	body, _ := json.Marshal(loginPayload)

	req, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName+"/login", bytes.NewReader(body))
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
