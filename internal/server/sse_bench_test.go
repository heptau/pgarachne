package server

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lib/pq"
)

func TestParseChannels(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		max     int
		want    []string
		wantErr bool
	}{
		{"single", "events", 8, []string{"events"}, false},
		{"multiple", "a,b,c", 8, []string{"a", "b", "c"}, false},
		{"dedup", "a,b,a", 8, []string{"a", "b"}, false},
		{"trim spaces", " a , b ", 8, []string{"a", "b"}, false},
		{"quoted", `"my channel"`, 8, []string{"my channel"}, false},
		{"quoted dedup", `"x","x"`, 8, []string{"x"}, false},
		{"empty", "", 8, nil, true},
		{"only commas", ",,,", 8, nil, true},
		{"too many", "a,b,c", 2, nil, true},
		{"invalid name", "bad-channel", 8, nil, true},
		{"empty quoted", `""`, 8, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseChannels(tc.raw, tc.max)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseChannels(%q, %d) err=%v, wantErr=%v", tc.raw, tc.max, err, tc.wantErr)
			}
			if !tc.wantErr {
				if len(got) != len(tc.want) {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
				for i := range tc.want {
					if got[i] != tc.want[i] {
						t.Errorf("got[%d]=%q, want %q", i, got[i], tc.want[i])
					}
				}
			}
		})
	}
}

func TestNormalizeChannelName(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"events", "events", false},
		{"user_events", "user_events", false},
		{`"my channel"`, "my channel", false},
		{`"weird""name"`, `weird"name`, false},
		{"", "", true},
		{"bad-name", "", true},
		{`""`, "", true},
	}
	for _, tc := range cases {
		got, err := normalizeChannelName(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("normalizeChannelName(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
		}
		if err == nil && got != tc.want {
			t.Errorf("normalizeChannelName(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestEventName(t *testing.T) {
	cases := []struct {
		ev   pq.ListenerEventType
		want string
	}{
		{pq.ListenerEventConnected, "connected"},
		{pq.ListenerEventDisconnected, "disconnected"},
		{pq.ListenerEventReconnected, "reconnected"},
		{pq.ListenerEventConnectionAttemptFailed, "connection_attempt_failed"},
		{pq.ListenerEventType(99), "other"},
	}
	for _, tc := range cases {
		if got := eventName(tc.ev); got != tc.want {
			t.Errorf("eventName(%d) = %q; want %q", tc.ev, got, tc.want)
		}
	}
}

func TestWriteSSEData(t *testing.T) {
	w := httptest.NewRecorder()
	msg := sseMessage{channel: "events", data: map[string]int{"id": 1}, dbName: "testdb"}
	n, err := writeSSEData(w, msg)
	if err != nil {
		t.Fatalf("writeSSEData: %v", err)
	}
	body := w.Body.String()
	if !strings.HasPrefix(body, "data: ") {
		t.Errorf("output does not start with 'data: ': %q", body)
	}
	if !strings.HasSuffix(body, "\n\n") {
		t.Errorf("output does not end with \\n\\n: %q", body)
	}
	if n != len(body) {
		t.Errorf("returned n=%d but wrote %d bytes", n, len(body))
	}
}

func TestWriteSSEComment(t *testing.T) {
	w := httptest.NewRecorder()
	if err := writeSSEComment(w, "ping"); err != nil {
		t.Fatalf("writeSSEComment: %v", err)
	}
	if got := w.Body.String(); got != ": ping\n\n" {
		t.Errorf("got %q; want \": ping\\n\\n\"", got)
	}
}

// BenchmarkParseChannels measures the cost of parsing the
// ?channels=a,b,c query parameter on every SSE connect. The function
// runs on the request hot path, so the cost matters for fan-out
// workloads (one operator opening many concurrent SSE connections).
// The bench varies the input size and a duplicate-heavy case to
// exercise the dedupe map.
func BenchmarkParseChannels(b *testing.B) {
	cases := []struct {
		name string
		raw  string
		max  int
	}{
		{"1_channel", "events", 32},
		{"8_channels", "a,b,c,d,e,f,g,h", 32},
		{"32_channels", strings.Repeat("ch,", 31) + "ch", 32},
		{"8_with_dupes", "a,b,a,b,a,b,a,b", 32},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := parseChannels(tc.raw, tc.max); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkNormalizeChannelName measures the cost of validating and
// unquoting a single channel name. The regex anchors are tuned to
// short-circuit on plain identifiers, so the bench covers both the
// "easy" (unquoted) and "expensive" (quoted with embedded quotes)
// branches.
func BenchmarkNormalizeChannelName(b *testing.B) {
	cases := []struct {
		name string
		raw  string
	}{
		{"plain", "events"},
		{"underscore", "user_events"},
		{"quoted", `"my channel"`},
		{"quoted_with_quote", `"weird""name"`},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := normalizeChannelName(tc.raw); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkParseNotifyPayload measures the cost of detecting a JSON
// object in a PostgreSQL NOTIFY extra payload. The function is called
// on every NOTIFY message that the SSE listener receives, so even
// minor allocations show up in the profile.
func BenchmarkParseNotifyPayload(b *testing.B) {
	cases := []struct {
		name    string
		payload string
	}{
		{"plain_string", "user logged in"},
		{"short_json", `{"id":1}`},
		{"rich_json", `{"event":"login","user_id":42,"ts":"2026-01-01T00:00:00Z","payload":{"a":1,"b":2}}`},
		{"malformed_json", `{"id":`},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = parseNotifyPayload(tc.payload)
			}
		})
	}
}

// BenchmarkEventName measures the cost of mapping pq.ListenerEventType
// to the stable string label that the sseEventsCounter Prometheus
// metric uses. The switch is small but it runs on every pq.Listener
// event, so it is on the SSE hot path.
func BenchmarkEventName(b *testing.B) {
	events := []pq.ListenerEventType{
		pq.ListenerEventConnected,
		pq.ListenerEventDisconnected,
		pq.ListenerEventReconnected,
		pq.ListenerEventConnectionAttemptFailed,
	}
	for _, ev := range events {
		b.Run(fmt.Sprintf("ev_%d", ev), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = eventName(ev)
			}
		})
	}
}
