package server

import "encoding/json"

type JSONRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
	ID      interface{}            `json:"id"`
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

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type LoginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginDBResponse struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	DBRole      string `json:"db_role"`
	TOTPEnabled bool   `json:"totp_enabled"`
}
