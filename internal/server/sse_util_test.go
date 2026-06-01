package server

import (
	"net/http/httptest"
	"testing"
)

// TestWriteSSEDataUnmarshalableData covers the marshal-error branch that the
// happy-path test in sse_bench_test.go cannot reach: nothing may be written
// and the reported byte count must stay zero so the bytes-sent metric is not
// inflated by frames the client never received.
func TestWriteSSEDataUnmarshalableData(t *testing.T) {
	rec := httptest.NewRecorder()
	msg := sseMessage{channel: "ch", data: make(chan int)}

	n, err := writeSSEData(rec, msg)
	if err == nil {
		t.Fatal("expected a marshal error for unmarshalable data, got nil")
	}
	if n != 0 {
		t.Errorf("byte count = %d; want 0 on error", n)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q; want nothing written on marshal error", rec.Body.String())
	}
}
