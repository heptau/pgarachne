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
	callReq, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName+"/api.hello_world", bytes.NewReader(callBody))
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
	callReq, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName+"/api.hello_world", bytes.NewReader(callBody))
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
		"method":  "api.hello_world",
		"params":  map[string]interface{}{},
		"id":      3,
	}
	callBody, _ := json.Marshal(callPayload)
	callReq, err := http.NewRequest(http.MethodPost, env.httpServer.URL+"/api/"+env.dbName+"/api.hello-world", bytes.NewReader(callBody))
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
