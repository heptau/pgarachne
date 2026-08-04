package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
)

// Sentinel errors returned by setupRequestTx. The JSON-RPC and MCP handlers
// map them to their own response shapes (HTTP status + JSON-RPC error object
// vs. always-200 MCP tool error).
var (
	errIdempotencyDuplicate   = errors.New("request already processed")
	errIdempotencyCheckFailed = errors.New("idempotency check failed")
	errSetRoleFailed          = errors.New("set role failed")
)

// setupRequestTx applies the per-request transaction setup shared by the
// JSON-RPC and MCP handlers, in this order:
//
//  1. SET LOCAL app.api_prefix — so capabilities() and
//     generate_openapi_spec() build correct endpoint URLs. Non-fatal: on
//     failure SQL falls back to the default 'db' prefix. SET LOCAL scopes the
//     value to this transaction, so the GUC does not leak across pooled
//     connections.
//  2. Idempotency-key check (when idempotencyKey != "") — intentionally
//     inside the transaction so the key is not persisted when the function
//     call rolls back. For JWT/API-token auth this runs as the service user
//     (DB_USER) before SET LOCAL ROLE, so no extra grants are needed on the
//     client role. For direct auth the check runs as the authenticated user;
//     the operator must grant EXECUTE ON FUNCTION
//     pgarachne.save_idempotency_key to that role if idempotency keys are
//     required.
//  3. SET LOCAL ROLE (when dbRole != "") — switches to the authenticated role
//     so Row-Level Security and function permissions are enforced correctly.
//     Skipped for direct auth (dbRole == ""), where the connection is already
//     authenticated as the user.
func (s *Server) setupRequestTx(ctx context.Context, tx *sql.Tx, dbRole, idempotencyKey string) error {
	// set_config(name, value, is_local => true) is the function form of
	// SET LOCAL, so the value travels as a bind parameter instead of being
	// spliced into SQL text.
	if _, err := tx.ExecContext(ctx,
		`SELECT set_config('app.api_prefix', $1, true)`, s.Cfg.APIPrefix,
	); err != nil {
		slog.Warn("Failed to SET LOCAL app.api_prefix", "value", s.Cfg.APIPrefix, "error", err)
	}

	if idempotencyKey != "" {
		var saved bool
		if err := tx.QueryRowContext(ctx,
			`SELECT pgarachne.save_idempotency_key($1)`, idempotencyKey,
		).Scan(&saved); err != nil {
			return fmt.Errorf("%w: %v", errIdempotencyCheckFailed, err)
		}
		if !saved {
			return errIdempotencyDuplicate
		}
	}

	if dbRole != "" {
		// SET ROLE x is defined as SET role = 'x', so set_config('role', …,
		// true) switches the role for the rest of this transaction exactly as
		// SET LOCAL ROLE would — but with the role name bound as a parameter,
		// which removes identifier quoting from the picture entirely. The
		// server-side privilege check on the target role is unchanged.
		if _, err := tx.ExecContext(ctx, `SELECT set_config('role', $1, true)`, dbRole); err != nil {
			return fmt.Errorf("%w: %v", errSetRoleFailed, err)
		}
	}

	return nil
}

// rollbackQuietly rolls back tx, ignoring sql.ErrTxDone — the expected result
// when the transaction was already committed. Intended for
// `defer rollbackQuietly(tx)`.
func rollbackQuietly(tx *sql.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		slog.Debug("Transaction rollback failed", "error", err)
	}
}
