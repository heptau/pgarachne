package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/heptau/pgarachne/internal/config"
)

func TestLoginLimiterBlockedCountsOnlyFailures(t *testing.T) {
	l := newLoginLimiter(2, 50*time.Millisecond)

	if l.Blocked("key") {
		t.Fatal("unknown key should not be blocked")
	}
	l.RecordFailure("key")
	if l.Blocked("key") {
		t.Fatal("one failure is below the limit of 2")
	}
	l.RecordFailure("key")
	if !l.Blocked("key") {
		t.Fatal("two failures should exhaust the limit of 2")
	}
	if l.Blocked("other") {
		t.Fatal("a different key has its own budget")
	}

	time.Sleep(60 * time.Millisecond)
	if l.Blocked("key") {
		t.Fatal("failures older than the window should no longer count")
	}
}

func TestLoginLimiterBlockedFailsClosedWhenFull(t *testing.T) {
	l := newLoginLimiter(5, time.Minute)
	l.maxEntries = 1
	l.RecordFailure("first")
	if !l.Blocked("second") {
		t.Fatal("unknown key should be blocked when the map is full (fail closed)")
	}
	if l.Blocked("first") {
		t.Fatal("tracked key below its limit should not be blocked")
	}
}

// newThrottleTestServer points the server at a port nothing listens on, so
// every Basic-Auth attempt fails at connect time regardless of the local
// PostgreSQL setup (e.g. Postgres.app's trust auth).
func newThrottleTestServer(t *testing.T, jwtSecret string) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		DBHost:              "127.0.0.1",
		DBPort:              1,
		DBUser:              "pgarachne",
		DBSSLMode:           "disable",
		JWTSecret:           jwtSecret,
		JWTExpiryHours:      1,
		MaxRequestBytes:     1 << 20,
		APIPrefix:           "db",
		LoginRateLimit:      2,
		LoginRateLimitPerIP: 100,
		LoginRateWindow:     time.Minute,
	}
	ts := httptest.NewServer(New(cfg).buildRouter())
	t.Cleanup(ts.Close)
	return ts
}

func postJSONRPC(t *testing.T, url, authHeader, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestBasicAuthFailuresAreRateLimited(t *testing.T) {
	ts := newThrottleTestServer(t, "")
	body := `{"jsonrpc":"2.0","method":"capabilities","id":1}`
	basic := "Basic " + "YWxpY2U6d3Jvbmc=" // alice:wrong

	for i := 0; i < 2; i++ {
		resp := postJSONRPC(t, ts.URL+"/db/mydb/jsonrpc", basic, body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want 401", i+1, resp.StatusCode)
		}
	}

	resp := postJSONRPC(t, ts.URL+"/db/mydb/jsonrpc", basic, body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("attempt after limit: status = %d, want 429", resp.StatusCode)
	}

	// The budget is shared across endpoints.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/db/mydb/openapi.json", nil)
	req.Header.Set("Authorization", basic)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("GET openapi.json after limit: status = %d, want 429", resp.StatusCode)
	}

	// A different username from the same IP still has its own budget.
	resp = postJSONRPC(t, ts.URL+"/db/mydb/jsonrpc", "Basic Ym9iOndyb25n", body) // bob:wrong
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("other user: status = %d, want 401", resp.StatusCode)
	}
}

func TestGetJWTDisabledWithoutSecret(t *testing.T) {
	ts := newThrottleTestServer(t, "")
	resp := postJSONRPC(t, ts.URL+"/db/mydb/jsonrpc", "",
		`{"jsonrpc":"2.0","method":"get_jwt","params":{"login":"a","password":"b"},"id":1}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	var out JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Error == nil || out.Error.Code != -32601 || !strings.Contains(out.Error.Message, "JWT_SECRET") {
		t.Fatalf("error = %+v, want -32601 mentioning JWT_SECRET", out.Error)
	}
}

func TestMissingAuthorizationRejectedBeforeDBConnect(t *testing.T) {
	ts := newThrottleTestServer(t, "")
	resp := postJSONRPC(t, ts.URL+"/db/mydb/jsonrpc", "", `{"jsonrpc":"2.0","method":"capabilities","id":1}`)
	resp.Body.Close()
	// 401, not 503: the unreachable database must not even be contacted.
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}
