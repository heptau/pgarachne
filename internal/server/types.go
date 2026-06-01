package server

import "encoding/json"

// Length limits enforced at the JSON-RPC edge. They are intentionally
// conservative — large enough for any realistic caller, small enough to
// bound the per-request CPU cost of validation, logging, and storage
// in the rate limiter and (where applicable) the database.
const (
	// MaxLoginLength matches PostgreSQL's NAMEDATALEN-1 (63). Usernames
	// longer than this cannot exist in the database, so accepting them
	// from the wire would be wasted work.
	MaxLoginLength = 63
	// MaxPasswordLength covers bcrypt (72) with headroom for any future
	// hashing scheme. The password is never stored — it goes straight
	// into the libpq connection string — so the limit exists to keep
	// the rate-limiter key and slog calls from churning through
	// arbitrarily long strings.
	MaxPasswordLength = 1024
	// MaxMethodLength bounds "schema.function" style names. PostgreSQL
	// identifiers max out at NAMEDATALEN-1 = 63 each, so 256 is more
	// than enough headroom for any sensible deployment.
	MaxMethodLength = 256
	// MaxIdempotencyKeyLength keeps pgarachne.requests free of garbage.
	// 256 characters is well beyond any reasonable UUID or hash.
	MaxIdempotencyKeyLength = 256
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
	// IdempotencyKey is an optional top-level extension field (not part of JSON-RPC 2.0
	// spec, but permitted — unknown fields are ignored by compliant implementations).
	// When present, the server calls pgarachne.save_idempotency_key before executing
	// the target function. If the key was already used in a previous successful call,
	// the request is rejected with HTTP 409 Conflict.
	// Placed at the top level intentionally so that params is passed to the SQL
	// function without any modification or stripping.
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      interface{}     `json:"id"`
}

// MarshalJSON ensures JSONRPC is always serialised as "2.0" even when the
// struct literal omitted it. JSON-RPC 2.0 §5 requires this field in every
// response object.
func (r JSONRPCResponse) MarshalJSON() ([]byte, error) {
	type alias JSONRPCResponse
	a := alias(r)
	if a.JSONRPC == "" {
		a.JSONRPC = "2.0"
	}
	return json.Marshal(a)
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type LoginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}
