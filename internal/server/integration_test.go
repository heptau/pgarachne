package server

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/heptau/pgarachne/internal/config"
	"github.com/heptau/pgarachne/internal/database"
	_ "github.com/lib/pq"
	promtest "github.com/prometheus/client_golang/prometheus/testutil"
)

type testEnv struct {
	cfg           *config.Config
	dbName        string
	testUser      string
	testPass      string
	tokenUser     string
	server        *Server
	httpServer    *httptest.Server
	metricsServer *httptest.Server
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
		DBHost:            dbHost,
		DBPort:            port,
		DBUser:            dbUser,
		DBSSLMode:         getenvDefault("DB_SSLMODE", "disable"),
		DBSSLRootCert:     os.Getenv("DB_SSLROOTCERT"),
		DBSSLCert:         os.Getenv("DB_SSLCERT"),
		DBSSLKey:          os.Getenv("DB_SSLKEY"),
		HTTPPort:          "0",
		JWTSecret:         jwtSecret,
		JWTExpiryHours:    1,
		AllowedOrigins:    []string{"*"},
		LogLevel:          "ERROR",
		LogOutput:         "stdout",
		LoginRateLimit:    getenvIntDefault("LOGIN_RATE_LIMIT", 5),
		LoginRateWindow:   getenvDurationDefault("LOGIN_RATE_WINDOW", time.Minute),
		MaxRequestBytes:   getenvInt64Default("MAX_REQUEST_BYTES", 2*1024*1024),
		MetricsEnabled:    true,
		MetricsListenAddr: "127.0.0.1:9090",
		APIPrefix:         getenvDefault("API_PREFIX", "db"),
		SSEMaxChannels:    getenvIntDefault("SSE_MAX_CHANNELS", 8),
		SSEHeartbeat:      getenvDurationDefault("SSE_HEARTBEAT", 20*time.Second),
		SSEIdleTimeout:    getenvDurationDefault("SSE_IDLE_TIMEOUT", 90*time.Second),
		SSEMaxClients:     getenvIntDefault("SSE_MAX_CLIENTS", 1000),
		SSEClientBuffer:   getenvIntDefault("SSE_CLIENT_BUFFER", 64),
		SSESendTimeout:    getenvDurationDefault("SSE_SEND_TIMEOUT", 2*time.Second),
	}

	gin.SetMode(gin.TestMode)
	srv := New(cfg)
	ts := httptest.NewServer(srv.buildRouter())
	metricsServer := httptest.NewServer(srv.buildMetricsRouter())

	return &testEnv{
		cfg:           cfg,
		dbName:        dbName,
		testUser:      getenvDefault("TEST_USER", "pgarachne_test_user"),
		testPass:      getenvDefault("TEST_PASSWORD", "pgarachne_test_password"),
		tokenUser:     dbUser,
		server:        srv,
		httpServer:    ts,
		metricsServer: metricsServer,
	}
}

func (e *testEnv) close() {
	if e.httpServer != nil {
		e.httpServer.Close()
	}
	if e.metricsServer != nil {
		e.metricsServer.Close()
	}
	if e.server != nil {
		// Mirror the production shutdown sequence so the test leaves
		// no pq.Listener or sql.DB sockets open. We use a short deadline
		// so a stuck Close doesn't hang the whole test binary.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = e.server.sseHub.Shutdown(ctx)
	}
	database.CloseAll()
}

// apiURL returns the canonical JSON-RPC endpoint URL for the test database.
func (e *testEnv) apiURL() string {
	return e.httpServer.URL + "/" + e.cfg.APIPrefix + "/" + e.dbName + "/jsonrpc"
}

// sseURL returns the canonical SSE endpoint URL for the test database.
func (e *testEnv) sseURL() string {
	return e.httpServer.URL + "/" + e.cfg.APIPrefix + "/" + e.dbName + "/sse"
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
	callReq, err := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(callBody))
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
	callReq, err := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(callBody))
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
	env := requireEnforcedPasswordAuth(t)
	defer env.close()

	_, err := loginAndGetToken(env, env.testUser, "wrong_password")
	if err == nil {
		t.Fatalf("expected login failure with invalid credentials")
	}
}

// TestLoginRejectsOversizedCredentials verifies that login and password
// longer than the documented limits are rejected with HTTP 400 before
// the request reaches the database, rate limiter, or the rate-limit
// counter. This is the DoS protection from internal/server/types.go.
func TestLoginRejectsOversizedCredentials(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	oversizedLogin := strings.Repeat("a", MaxLoginLength+1)
	oversizedPassword := strings.Repeat("p", MaxPasswordLength+1)

	// Oversized login. Should not be rate-limited (size check is first).
	if status := loginAndGetStatus(env, oversizedLogin, "any"); status != http.StatusBadRequest {
		t.Errorf("oversized login: status = %d, want %d", status, http.StatusBadRequest)
	}
	// Oversized password.
	if status := loginAndGetStatus(env, env.testUser, oversizedPassword); status != http.StatusBadRequest {
		t.Errorf("oversized password: status = %d, want %d", status, http.StatusBadRequest)
	}
}

// TestJSONRPCRejectsOversizedMethod verifies that the "method" field is
// bounded before any database work is done.
func TestJSONRPCRejectsOversizedMethod(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	oversizedMethod := strings.Repeat("a", MaxMethodLength+1)
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  oversizedMethod,
		"params":  map[string]interface{}{},
		"id":      1,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("oversized method: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestJSONRPCRejectsOversizedIdempotencyKey verifies that a too-long
// idempotencyKey is rejected before save_idempotency_key is called.
func TestJSONRPCRejectsOversizedIdempotencyKey(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	oversizedKey := strings.Repeat("k", MaxIdempotencyKeyLength+1)
	payload := map[string]interface{}{
		"jsonrpc":        "2.0",
		"method":         "api.hello_world",
		"params":         map[string]interface{}{},
		"id":             1,
		"idempotencyKey": oversizedKey,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("oversized idempotencyKey: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
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
	callReq, err := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(callBody))
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
	callReq, err := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(callBody))
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
	env := requireEnforcedPasswordAuth(t)
	defer env.close()

	// The limiter is configured when the server is built (env vars set after
	// that point have no effect), so exhaust the configured per-(IP, username)
	// budget and verify the next attempt is throttled.
	limit := env.cfg.LoginRateLimit
	if limit <= 0 {
		t.Skip("login rate limiting disabled in test config")
	}

	for i := 1; i <= limit; i++ {
		status := loginAndGetStatus(env, env.testUser, "wrong_password")
		if status != http.StatusUnauthorized {
			t.Fatalf("login attempt %d status = %d, want %d", i, status, http.StatusUnauthorized)
		}
	}

	status := loginAndGetStatus(env, env.testUser, "wrong_password")
	if status != http.StatusTooManyRequests {
		t.Fatalf("login attempt %d status = %d, want %d", limit+1, status, http.StatusTooManyRequests)
	}
}

func TestMaxRequestBodySize(t *testing.T) {
	t.Setenv("MAX_REQUEST_BYTES", "128")

	env := requireTestEnv(t)
	defer env.close()

	// Create an oversized JSON-RPC payload
	oversized := bytes.Repeat([]byte("a"), 256)
	body := append([]byte(`{"jsonrpc":"2.0","id":1,"method":"get_jwt","params":{"login":"x","password":"`), oversized...)
	body = append(body, []byte(`"}}`)...)

	req, err := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(body))
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

	req, err := http.NewRequest(http.MethodGet, env.sseURL(), nil)
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

	req, err := http.NewRequest(http.MethodGet, env.sseURL()+"?channels=one,two", nil)
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

	req, err := http.NewRequest(http.MethodGet, env.sseURL()+"?channels=limit_channel", nil)
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

func TestSSEMetrics(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	resp, _ := openSSE(t, env, token, "metrics_channel")
	defer resp.Body.Close()

	metricsResp, err := http.Get(env.metricsServer.URL + "/metrics")
	if err != nil {
		t.Fatalf("metrics request failed: %v", err)
	}
	defer metricsResp.Body.Close()

	if metricsResp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d", metricsResp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(metricsResp.Body)
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	metrics := string(body)

	if !strings.Contains(metrics, "pgarachne_sse_clients") {
		t.Fatalf("missing pgarachne_sse_clients metric")
	}
	if !strings.Contains(metrics, "pgarachne_sse_channels") {
		t.Fatalf("missing pgarachne_sse_channels metric")
	}
	if !strings.Contains(metrics, "pgarachne_sse_client_drops_total") {
		t.Fatalf("missing pgarachne_sse_client_drops_total metric")
	}
	// The three new metrics must be exposed even before any traffic —
	// the counter is incremented from inside the listener goroutine and
	// the byte counter from the per-client handler, so we just verify
	// the names are present in the registry.
	if !strings.Contains(metrics, "pgarachne_sse_events_forwarded_total") {
		t.Fatalf("missing pgarachne_sse_events_forwarded_total metric")
	}
	if !strings.Contains(metrics, "pgarachne_sse_bytes_sent_total") {
		t.Fatalf("missing pgarachne_sse_bytes_sent_total metric")
	}
	if !strings.Contains(metrics, "pgarachne_sse_listen_errors_total") {
		t.Fatalf("missing pgarachne_sse_listen_errors_total metric")
	}
}

// TestSSEEventsForwardedCounter is an end-to-end check that a NOTIFY on
// the database actually bumps the pgarachne_sse_events_forwarded_total
// counter. We send N notifications and assert the counter increased by
// at least N — not exactly N, because other tests running in parallel
// against the same database would share the counter.
func TestSSEEventsForwardedCounter(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	channel := "sse_events_counter_test"
	resp, reader := openSSE(t, env, token, channel)
	defer resp.Body.Close()

	connStr := buildConnStr(env.cfg, env.dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	before := readCounter(t, env.metricsServer.URL, "pgarachne_sse_events_forwarded_total")

	const n = 3
	for i := 0; i < n; i++ {
		if _, err := db.Exec(fmt.Sprintf("NOTIFY %s, '{\"i\":%d}'", channel, i)); err != nil {
			t.Fatalf("notify: %v", err)
		}
		// Drain the event so the listener goroutine processes it before
		// we read the counter — without this, the test can race and
		// see a smaller delta than expected.
		_ = readSSEData(t, reader, 3*time.Second)
	}

	// Allow the metric to settle (Prometheus counters are updated in
	// the listener goroutine; the broadcast path increments before
	// the per-client handler returns, so this is normally instant).
	time.Sleep(100 * time.Millisecond)

	after := readCounter(t, env.metricsServer.URL, "pgarachne_sse_events_forwarded_total")
	if after-before < float64(n) {
		t.Errorf("pgarachne_sse_events_forwarded_total delta = %v, want >= %d (before=%v after=%v)", after-before, n, before, after)
	}
}

// readCounter scrapes the metrics endpoint and returns the sum of all
// label combinations of the named counter. Returns 0 if the counter is
// not yet present in the registry (which is fine for a fresh start).
func readCounter(t *testing.T, metricsURL, name string) float64 {
	t.Helper()
	resp, err := http.Get(metricsURL + "/metrics")
	if err != nil {
		t.Fatalf("metrics request failed: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	var total float64
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, name) {
			continue
		}
		// Skip the HELP and TYPE lines.
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(parts[len(parts)-1], 64)
		if err != nil {
			continue
		}
		total += v
	}
	return total
}

func TestCoreMetrics(t *testing.T) {
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
		"id":      42,
	}
	callBody, _ := json.Marshal(callPayload)
	callReq, err := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(callBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	callReq.Header.Set("Content-Type", "application/json")
	callReq.Header.Set("Authorization", "Bearer "+token)

	callResp, err := http.DefaultClient.Do(callReq)
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	callResp.Body.Close()

	metricsResp, err := http.Get(env.metricsServer.URL + "/metrics")
	if err != nil {
		t.Fatalf("metrics request failed: %v", err)
	}
	defer metricsResp.Body.Close()

	body, err := io.ReadAll(metricsResp.Body)
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	metrics := string(body)

	if !strings.Contains(metrics, "pgarachne_http_requests_total") {
		t.Fatalf("missing pgarachne_http_requests_total metric")
	}
	if !strings.Contains(metrics, "pgarachne_http_request_duration_seconds") {
		t.Fatalf("missing pgarachne_http_request_duration_seconds metric")
	}
	if !strings.Contains(metrics, "pgarachne_auth_requests_total") {
		t.Fatalf("missing pgarachne_auth_requests_total metric")
	}
	if !strings.Contains(metrics, "pgarachne_login_attempts_total") {
		t.Fatalf("missing pgarachne_login_attempts_total metric")
	}
	if !strings.Contains(metrics, "pgarachne_jsonrpc_requests_total") {
		t.Fatalf("missing pgarachne_jsonrpc_requests_total metric")
	}
}

func TestMetricsNotExposedOnMainAPI(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	resp, err := http.Get(env.httpServer.URL + "/metrics")
	if err != nil {
		t.Fatalf("metrics request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestOpenAPISpec verifies that GET /{prefix}/:database/openapi.json
// returns a valid OpenAPI 3.1 document, generated on the fly from
// pgarachne.generate_openapi_spec. The spec is served as
// application/json (what OpenAPI tooling expects), is reachable without
// authentication (it only describes method names, never data), and
// embeds the per-method x-pgarachne-methods extension.
func TestOpenAPISpec(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	url := env.httpServer.URL + "/" + env.cfg.APIPrefix + "/" + env.dbName + "/openapi.json"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json prefix", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("unmarshal spec: %v", err)
	}

	if got, want := spec["openapi"], "3.1.0"; got != want {
		t.Errorf("openapi = %v, want %v", got, want)
	}

	info, ok := spec["info"].(map[string]interface{})
	if !ok {
		t.Fatal("info block missing or not an object")
	}
	if title, _ := info["title"].(string); !strings.Contains(title, env.dbName) {
		t.Errorf("info.title = %q, expected to contain %q", title, env.dbName)
	}

	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("paths block missing or not an object")
	}
	// Exactly one path — the JSON-RPC endpoint — keyed by the configured
	// prefix + database name + /jsonrpc.
	wantPath := "/" + env.cfg.APIPrefix + "/" + env.dbName + "/jsonrpc"
	if _, ok := paths[wantPath]; !ok {
		t.Errorf("paths missing key %q (have: %v)", wantPath, mapKeys(paths))
	}

	components, ok := spec["components"].(map[string]interface{})
	if !ok {
		t.Fatal("components block missing")
	}
	secSchemes, _ := components["securitySchemes"].(map[string]interface{})
	bearer, _ := secSchemes["BearerAuth"].(map[string]interface{})
	if bearer == nil {
		t.Errorf("components.securitySchemes.BearerAuth missing (have: %v)", secSchemes)
	} else if bearer["type"] != "http" || bearer["scheme"] != "bearer" {
		t.Errorf("BearerAuth = %v, want {type:http, scheme:bearer}", bearer)
	}
}

// mapKeys returns the keys of a map[string]interface{} as a slice, for
// test diagnostics. Helper for TestOpenAPISpec so the error message can
// list the actual paths returned by the server.
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestLegacyAPIPathRemoved verifies that the pre-2.0 POST /api/:database path
// (a 307 redirect to /{prefix}/:database/jsonrpc in 1.x) no longer exists.
func TestLegacyAPIPathRemoved(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	body := []byte(`{"jsonrpc":"2.0","method":"get_jwt","params":{},"id":1}`)
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

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (legacy path removed in 2.0)", resp.StatusCode, http.StatusNotFound)
	}
}

// TestLegacySSEPathRemoved verifies that the pre-2.0 GET /sse/:database path
// (a 307 redirect to /{prefix}/:database/sse in 1.x) no longer exists.
func TestLegacySSEPathRemoved(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	req, err := http.NewRequest(http.MethodGet, env.httpServer.URL+"/sse/"+env.dbName+"?channels=test_ch", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (legacy path removed in 2.0)", resp.StatusCode, http.StatusNotFound)
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

// TestCapabilitiesEndpointUsesAPIPrefix verifies that pgarachne.capabilities
// builds endpoint URLs using the GUC app.api_prefix that the Go server sets
// from its APIPrefix config. Two separate server instances with different
// prefixes must produce different endpoint strings.
func TestCapabilitiesEndpointUsesAPIPrefix(t *testing.T) {
	envDefault := requireTestEnvWithAPIPrefix(t, "db")
	envCustom := requireTestEnvWithAPIPrefix(t, "myapp")

	for _, tc := range []struct {
		name    string
		env     *testEnv
		prefix  string
		wantSub string
	}{
		{"default db prefix", envDefault, "db", "/db/"},
		{"custom prefix", envCustom, "myapp", "/myapp/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := loginAndGetTokenOrSkip(t, tc.env)
			endpoints, err := fetchCapabilitiesEndpoints(tc.env, token)
			if err != nil {
				t.Fatalf("fetchCapabilitiesEndpoints failed: %v", err)
			}
			if len(endpoints) == 0 {
				t.Fatal("no capabilities returned")
			}
			for _, ep := range endpoints {
				if !strings.Contains(ep, tc.wantSub) {
					t.Errorf("endpoint %q does not contain %q", ep, tc.wantSub)
				}
			}
		})
	}
}

func requireTestEnvWithAPIPrefix(t *testing.T, prefix string) *testEnv {
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
		DBHost:            dbHost,
		DBPort:            port,
		DBUser:            dbUser,
		DBSSLMode:         getenvDefault("DB_SSLMODE", "disable"),
		DBSSLRootCert:     os.Getenv("DB_SSLROOTCERT"),
		DBSSLCert:         os.Getenv("DB_SSLCERT"),
		DBSSLKey:          os.Getenv("DB_SSLKEY"),
		HTTPPort:          "0",
		JWTSecret:         jwtSecret,
		JWTExpiryHours:    1,
		AllowedOrigins:    []string{"*"},
		LogLevel:          "ERROR",
		LogOutput:         "stdout",
		LoginRateLimit:    5,
		LoginRateWindow:   time.Minute,
		MaxRequestBytes:   2 * 1024 * 1024,
		MetricsEnabled:    true,
		MetricsListenAddr: "127.0.0.1:9090",
		APIPrefix:         prefix,
		SSEMaxChannels:    8,
		SSEHeartbeat:      20 * time.Second,
		SSEIdleTimeout:    60 * time.Second,
		SSEMaxClients:     64,
		SSEClientBuffer:   16,
		SSESendTimeout:    2 * time.Second,
	}

	gin.SetMode(gin.TestMode)
	srv := New(cfg)
	ts := httptest.NewServer(srv.buildRouter())

	t.Cleanup(func() {
		ts.Close()
	})

	return &testEnv{
		cfg:           cfg,
		dbName:        dbName,
		testUser:      getenvDefault("TEST_USER", "pgarachne_test_user"),
		testPass:      getenvDefault("TEST_PASSWORD", "pgarachne_test_password"),
		tokenUser:     dbUser,
		httpServer:    ts,
		metricsServer: httptest.NewServer(srv.buildMetricsRouter()),
	}
}

func loginAndGetTokenOrSkip(t *testing.T, env *testEnv) string {
	t.Helper()
	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Skipf("login failed (test user not set up): %v", err)
	}
	return token
}

func fetchCapabilitiesEndpoints(env *testEnv, token string) ([]string, error) {
	callPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "capabilities",
		"params":  map[string]interface{}{},
		"id":      4,
	}
	callBody, _ := json.Marshal(callPayload)
	callReq, err := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(callBody))
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
		Endpoint string `json:"endpoint"`
	}
	if err := json.Unmarshal(rpcResp.Result, &methods); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(methods))
	for _, m := range methods {
		out = append(out, m.Endpoint)
	}
	return out, nil
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
	callReq, err := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(callBody))
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
		"method":  "get_jwt",
		"params": map[string]string{
			"login":    login,
			"password": password,
		},
		"id": 1,
	}
	body, _ := json.Marshal(loginPayload)

	req, err := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(body))
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
		"method":  "get_jwt",
		"params": map[string]string{
			"login":    login,
			"password": password,
		},
		"id": 1,
	}
	body, _ := json.Marshal(loginPayload)

	req, err := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(body))
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

	req, err := http.NewRequest(http.MethodGet, env.sseURL()+"?channels="+url.QueryEscape(channels), nil)
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

// buildConnStrWithCreds is like buildConnStr but takes an explicit
// username and password. Used by requireEnforcedPasswordAuth to verify
// that the test DB actually rejects wrong passwords.
func buildConnStrWithCreds(cfg *config.Config, dbName, user, password string) string {
	return "host=" + cfg.DBHost +
		" port=" + strconv.Itoa(cfg.DBPort) +
		" user=" + user +
		" password=" + password +
		" dbname=" + dbName +
		" " + cfg.DBSSLParams()
}

// requireEnforcedPasswordAuth verifies that the test PostgreSQL instance
// actually rejects wrong passwords. Postgres.app (and some other local
// setups) ship with `trust` auth on localhost by default, which makes
// negative-login tests pass when they should fail. Tests that depend on
// password rejection must call this helper and t.Skip if the check
// reveals trust auth.
//
// On CI (where the standard Postgres Docker image uses scram-sha-256),
// this check passes and the test runs normally.
//
// Returns the test env so the caller can use it without a second call
// to requireTestEnv.
func requireEnforcedPasswordAuth(t *testing.T) *testEnv {
	t.Helper()

	env := requireTestEnv(t)

	// Open a fresh, separate connection with a definitely-wrong
	// password. The pgarachne service user is in cfg.DBUser; for
	// negative-login scenarios we want to authenticate as the test
	// user, which is the one whose password an attacker would guess.
	connStr := buildConnStrWithCreds(env.cfg, env.dbName, env.testUser, "this_is_definitely_wrong")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("open db with wrong password: %v", err)
	}
	defer db.Close()

	// Use a short context so a stuck connection does not hang the test.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err == nil {
		t.Skip("test PostgreSQL accepts any password (pg_hba.conf trust auth); negative-login tests cannot run")
	}

	return env
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

func getenvDurationDefault(key string, def time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return def
}

// TestSSEHubShutdownEmpty verifies the no-op path: shutting down a hub
// that never acquired any listener must return nil within the deadline
// and not block.
func TestSSEHubShutdownEmpty(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := env.server.sseHub.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown on empty hub returned error: %v", err)
	}
}

// TestSSEHubShutdownClosesActiveClient verifies the graceful path: an
// open SSE client is dropped when the hub is shut down. We assert on
// the server-side invariants — listener removed from the hub and the
// drop metric incremented with reason "shutdown" — rather than on the
// client-side EOF.
//
// Rationale: Go's http.Server keeps the underlying TCP connection alive
// after a chunked response terminator so HTTP/1.1 keep-alive can serve
// the next request. For SSE (which never sends Content-Length), the
// client therefore does not see EOF on the read side until the idle
// timeout elapses. The server, however, has already signalled
// termination by closing the client.done channel and removing the
// client from the listener. That is what graceful shutdown guarantees:
// the server side is fully unwound, even if the keep-alive socket is
// still open.
func TestSSEHubShutdownClosesActiveClient(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	resp, _ := openSSE(t, env, token, "shutdown_test_channel")
	defer resp.Body.Close()

	// Sanity: the hub now owns a listener for this database.
	env.server.sseHub.mu.Lock()
	listenerCount := len(env.server.sseHub.dbs)
	env.server.sseHub.mu.Unlock()
	if listenerCount != 1 {
		t.Fatalf("expected 1 active listener before shutdown, got %d", listenerCount)
	}

	// Capture drop metric for this database before shutdown.
	preShutdownDrops := promtest.ToFloat64(sseDropsCounter.WithLabelValues(env.dbName, "shutdown"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := env.server.sseHub.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	// Invariant 1: the hub's listener map is empty.
	env.server.sseHub.mu.Lock()
	listenerCount = len(env.server.sseHub.dbs)
	env.server.sseHub.mu.Unlock()
	if listenerCount != 0 {
		t.Fatalf("expected 0 listeners after shutdown, got %d", listenerCount)
	}

	// Invariant 2: a drop with reason "shutdown" was recorded for this db.
	postShutdownDrops := promtest.ToFloat64(sseDropsCounter.WithLabelValues(env.dbName, "shutdown"))
	if postShutdownDrops <= preShutdownDrops {
		t.Fatalf("expected shutdown drop counter to increase, pre=%v post=%v", preShutdownDrops, postShutdownDrops)
	}
}

// TestSSEHubShutdownIsIdempotent verifies that calling Shutdown twice
// does not panic and returns nil both times. Important because the
// production code path may call Shutdown once and unit tests may call
// env.close() which also shuts down — we must not double-close channels.
func TestSSEHubShutdownIsIdempotent(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := env.server.sseHub.Shutdown(ctx); err != nil {
		t.Fatalf("first Shutdown returned error: %v", err)
	}
	if err := env.server.sseHub.Shutdown(ctx); err != nil {
		t.Fatalf("second Shutdown returned error: %v", err)
	}
}
