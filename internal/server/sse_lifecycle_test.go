package server

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// startProductionHTTPServer serves env's router through newHTTPServer — the
// same http.Server (timeouts, shutdown hook) that Run() uses — instead of the
// bare httptest server the other integration tests talk to. It returns the
// server and its base URL. tweak, when non-nil, may adjust the server before
// it starts serving.
func startProductionHTTPServer(t *testing.T, env *testEnv, tweak func(*http.Server)) (*http.Server, string) {
	t.Helper()

	srv := env.server.newHTTPServer(env.server.buildRouter())
	if tweak != nil {
		tweak(srv)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	return srv, "http://" + ln.Addr().String()
}

func openSSEAt(t *testing.T, baseURL string, env *testEnv, token, channel string) (*http.Response, *bufio.Reader) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet,
		baseURL+"/"+env.cfg.APIPrefix+"/"+env.dbName+"/sse?channels="+channel, nil)
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

// TestSSEStreamOutlivesWriteTimeout pins that an SSE stream is not cut off by
// the server-wide WriteTimeout. That timeout is an absolute deadline for the
// whole response, so without a per-stream override every stream died once it
// elapsed (after one minute in production).
func TestSSEStreamOutlivesWriteTimeout(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	env.cfg.SSEHeartbeat = 50 * time.Millisecond
	// Shrink the production timeouts so the test does not take a minute.
	const timeout = 300 * time.Millisecond
	_, baseURL := startProductionHTTPServer(t, env, func(srv *http.Server) {
		srv.WriteTimeout = timeout
		srv.ReadTimeout = timeout
	})

	resp, reader := openSSEAt(t, baseURL, env, token, "write_timeout_test_channel")
	defer resp.Body.Close()

	// Keep reading heartbeats until well past the WriteTimeout.
	deadline := time.Now().Add(4 * timeout)
	pings := 0
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("stream ended after %d heartbeats, before outliving WriteTimeout: %v", pings, err)
		}
		if strings.HasPrefix(line, ": ping") {
			pings++
		}
	}
	if pings == 0 {
		t.Fatal("no heartbeat received")
	}
}

// TestShutdownDrainsActiveSSEClient pins that graceful shutdown finishes
// while an SSE client is connected. http.Server.Shutdown waits for active
// requests and an SSE stream never ends on its own, so without the shutdown
// hook the drain ran into its deadline and the rest of the cleanup was
// skipped.
func TestShutdownDrainsActiveSSEClient(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()

	token, err := loginAndGetToken(env, env.testUser, env.testPass)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	srv, baseURL := startProductionHTTPServer(t, env, nil)

	resp, reader := openSSEAt(t, baseURL, env, token, "shutdown_drain_test_channel")
	defer resp.Body.Close()
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("reading SSE preamble: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown with an active SSE client: %v", err)
	}
}

// TestCORSPreflightAllowsMCPHeaders pins that the headers every MCP request
// must carry pass the CORS preflight, so browser-based MCP clients can call
// the endpoint cross-origin.
func TestCORSPreflightAllowsMCPHeaders(t *testing.T) {
	ts := newProtocolTestServer(t)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/db/testdb/mcp", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", "https://client.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "authorization, content-type, mcp-method, mcp-name, mcp-protocol-version")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("preflight failed: %v", err)
	}
	defer resp.Body.Close()

	allowed := strings.ToLower(strings.Join(resp.Header.Values("Access-Control-Allow-Headers"), ","))
	for _, h := range []string{"mcp-protocol-version", "mcp-method", "mcp-name"} {
		if !strings.Contains(allowed, h) {
			t.Errorf("Access-Control-Allow-Headers = %q, missing %q", allowed, h)
		}
	}
}
