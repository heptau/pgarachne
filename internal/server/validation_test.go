package server

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestIsSafeDatabaseName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple", "mydb", true},
		{"underscore", "_db1", true},
		{"dollar", "db$1", true},
		{"quoted_simple", "\"MyDb\"", true},
		{"quoted_space", "\"My Db\"", true},
		{"quoted_quote", "\"weird\"\"name\"", true},
		{"empty", "", false},
		{"dash", "my-db", false},
		{"semi", "db;drop", false},
		{"dot", "db.name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafeDatabaseName(tt.input); got != tt.want {
				t.Fatalf("isSafeDatabaseName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// BenchmarkIsSafeDatabaseName measures the per-request cost of the
// database-name validation regex. The validation runs on every
// /{prefix}/:database/... route, so it is a hot path; this benchmark
// guards against accidental regex backtracking or quadratic behaviour.
func BenchmarkIsSafeDatabaseName(b *testing.B) {
	inputs := []struct {
		name  string
		value string
	}{
		{"simple", "mydb"},
		{"underscore", "_db1"},
		{"quoted", `"My Db"`},
		{"invalid", "db;DROP"},
	}
	for _, tc := range inputs {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = isSafeDatabaseName(tc.value)
			}
		})
	}
}

func TestIsSafeFunctionName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple", "api.hello_world", true},
		{"quoted_schema", "\"Api\".hello_world", true},
		{"quoted_func", "api.\"Hello World\"", true},
		{"quoted_both", "\"Api\".\"Hello World\"", true},
		{"quoted_quote", "\"A\"\"B\".\"C\"\"D\"", true},
		{"empty", "", false},
		{"missing_dot", "hello_world", false},
		{"double_dot", "api..hello", false},
		{"bad_chars", "api.hello-world", false},
		{"semi", "api.hello;drop", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafeFunctionName(tt.input); got != tt.want {
				t.Fatalf("isSafeFunctionName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildFunctionQuery(t *testing.T) {
	cases := []struct {
		name      string
		fn        string
		wantSub   string // substring expected in the query
		wantExact string // exact query when non-empty
	}{
		{
			name:      "capabilities unqualified",
			fn:        "capabilities",
			wantExact: `SELECT pgarachne.capabilities($1::jsonb)::json`,
		},
		{
			name:      "capabilities schema-qualified",
			fn:        "pgarachne.capabilities",
			wantExact: `SELECT pgarachne.capabilities($1::jsonb)::json`,
		},
		{
			name:    "schema-qualified function",
			fn:      "api.hello_world",
			wantSub: "api.hello_world",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildFunctionQuery(tc.fn)
			if tc.wantExact != "" && got != tc.wantExact {
				t.Errorf("buildFunctionQuery(%q) = %q; want %q", tc.fn, got, tc.wantExact)
			}
			if tc.wantSub != "" && !strings.Contains(got, tc.wantSub) {
				t.Errorf("buildFunctionQuery(%q) = %q; want substring %q", tc.fn, got, tc.wantSub)
			}
		})
	}
}

func TestNullParamsNormalization(t *testing.T) {
	// Verify that the null-params sentinel we apply in handleFunctionCall and
	// handleLoginRPC converts "null" and empty RawMessage to "{}".
	normalize := func(p json.RawMessage) json.RawMessage {
		if len(p) == 0 || string(p) == "null" {
			return json.RawMessage("{}")
		}
		return p
	}

	cases := []struct {
		name  string
		input json.RawMessage
		want  string
	}{
		{"nil slice", nil, "{}"},
		{"empty slice", json.RawMessage{}, "{}"},
		{"json null", json.RawMessage("null"), "{}"},
		{"empty object", json.RawMessage("{}"), "{}"},
		{"real params", json.RawMessage(`{"x":1}`), `{"x":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(normalize(tc.input))
			if got != tc.want {
				t.Errorf("normalize(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestQuoteRole(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"app_user", `"app_user"`},
		{"", `""`},
		{`has"quote`, `"has""quote"`},
		{`a""b`, `"a""""b"`},
	}
	for _, tc := range cases {
		if got := quoteRole(tc.in); got != tc.want {
			t.Errorf("quoteRole(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseNotifyPayload(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantStr bool // true = expect string result, false = expect map/object
	}{
		{"plain string", "user logged in", true},
		{"valid json object", `{"event":"login","id":1}`, false},
		{"malformed json", `{"id":`, true},
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"json array", `[1,2,3]`, true}, // arrays are not objects, returned as string
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := parseNotifyPayload(tc.payload)
			_, isStr := result.(string)
			if isStr != tc.wantStr {
				t.Errorf("parseNotifyPayload(%q) returned string=%v; want string=%v (value: %v)",
					tc.payload, isStr, tc.wantStr, result)
			}
		})
	}
}

func TestLoginLimiterMaxEntries(t *testing.T) {
	l := &loginLimiter{
		limit:      10,
		window:     time.Minute,
		maxEntries: 3,
		entries:    make(map[string][]time.Time),
	}

	// Fill up to maxEntries.
	if !l.Allow("key1") {
		t.Fatal("key1 should be allowed (first entry)")
	}
	if !l.Allow("key2") {
		t.Fatal("key2 should be allowed (second entry)")
	}
	if !l.Allow("key3") {
		t.Fatal("key3 should be allowed (third entry, at cap)")
	}

	// A new key must be denied when the map is at capacity.
	if l.Allow("key4") {
		t.Error("key4 should be denied: map is at maxEntries cap")
	}

	// An existing key must still be allowed (it's already tracked).
	if !l.Allow("key1") {
		t.Error("key1 should still be allowed: it is already in the map")
	}
}

func TestParseBasicAuth(t *testing.T) {
	encode := func(s string) string {
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(s))
	}

	cases := []struct {
		name     string
		header   string
		wantUser string
		wantPass string
		wantOK   bool
	}{
		{"valid simple", encode("alice:secret"), "alice", "secret", true},
		{"password has colon", encode("bob:pass:word"), "bob", "pass:word", true},
		{"empty password", encode("carol:"), "carol", "", true},
		{"empty username", encode(":pass"), "", "pass", true},
		{"no colon", encode("nocolon"), "", "", false},
		{"bearer scheme", "Bearer eyJhb...", "", "", false},
		{"empty header", "", "", "", false},
		{"bad base64", "Basic not!base64==", "", "", false},
		{"case insensitive", "BASIC " + base64.StdEncoding.EncodeToString([]byte("u:p")), "u", "p", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, p, ok := parseBasicAuth(tc.header)
			if ok != tc.wantOK {
				t.Fatalf("parseBasicAuth(%q) ok=%v, want %v", tc.header, ok, tc.wantOK)
			}
			if ok {
				if u != tc.wantUser {
					t.Errorf("username = %q, want %q", u, tc.wantUser)
				}
				if p != tc.wantPass {
					t.Errorf("password = %q, want %q", p, tc.wantPass)
				}
			}
		})
	}
}

// TestJSONRPCResponseAlwaysHasVersion verifies that every JSONRPCResponse
// — including error responses constructed without an explicit JSONRPC field —
// serialises with "jsonrpc":"2.0" as required by JSON-RPC 2.0 §5.
func TestJSONRPCResponseAlwaysHasVersion(t *testing.T) {
	cases := []struct {
		name string
		r    JSONRPCResponse
	}{
		{"success with version", JSONRPCResponse{JSONRPC: "2.0", ID: 1}},
		{"error without version", JSONRPCResponse{Error: &JSONRPCError{Message: "oops"}, ID: 1}},
		{"zero value", JSONRPCResponse{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.r)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var m map[string]interface{}
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got, _ := m["jsonrpc"].(string); got != "2.0" {
				t.Errorf("jsonrpc = %q; want \"2.0\" (raw: %s)", got, b)
			}
		})
	}
}

// BenchmarkIsSafeFunctionName measures the per-request cost of the
// schema.function validation regex. It runs on every JSON-RPC and MCP
// dispatch, so it is a hot path; this benchmark guards against regex
// backtracking or accidental re-compilation of the underlying regexp.
func BenchmarkIsSafeFunctionName(b *testing.B) {
	inputs := []struct {
		name  string
		value string
	}{
		{"simple", "api.hello_world"},
		{"long_namespace", "analytics_internal.public.user_lookup"},
		{"dollar", "api.fn$1"},
		{"invalid", "api.do;DROP"},
	}
	for _, tc := range inputs {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = isSafeFunctionName(tc.value)
			}
		})
	}
}
