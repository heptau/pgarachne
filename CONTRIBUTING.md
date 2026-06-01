# Contributing to PgArachne

Thanks for contributing.

## Development Basics

- Follow the project architecture: JSON-RPC over `POST /{prefix}/:database/jsonrpc`
  mapped to PostgreSQL functions with a single `jsonb` input. The `{prefix}`
  defaults to `db` and is configurable via `API_PREFIX`.
- Keep the security model intact (`SET LOCAL ROLE` and DB role-based access
  are intentional).
- When changing behaviour, verify whether updates are needed in both Go code
  and SQL (`sql/schema.sql`, `sql/mcp_functions.sql`).
- The `tools/call` MCP method has special error semantics: SQL failures are
  reported as `isError: true` in the result, **not** as JSON-RPC protocol
  errors. Do not change this without reading the MCP spec §tool-result.

## Formatting and Style

- Basic formatting rules are defined in `.editorconfig`.
- All code comments must be in English.
- Exception: comments in localised files may be in English or in the
  corresponding local language.
- Run `gofmt -s -w .` and `make lint` before opening a PR.

## Project Conventions

- The configurable JSON-RPC endpoint is `POST /{prefix}/:database/jsonrpc`.
  The default `API_PREFIX=db` makes the route `/db/:database/jsonrpc`.
- The SSE endpoint is `GET /{prefix}/:database/sse` with required
  `channels=...` query.
- The MCP endpoint is `POST /{prefix}/:database/mcp` (Streamable HTTP
  transport, protocol version `2024-11-05`).
- Authentication is the same Bearer token model across all three endpoints
  (JWT issued by `get_jwt`, or long-lived API token).
- `make release-local` builds and verifies release artifacts locally (no git, no push). `make release` runs that, then tags, pushes, creates the GitHub release, and updates the Homebrew tap — see `scripts/publish_release.sh`.
- Homebrew tap artefacts are generated into `dist/homebrew-tap/`.
- Docs are authored in `docs-src/` and built into `docs/`.

## Release Naming (macOS GUI ZIPs)

- `pgarachne-macos-amd64-app.zip`
- `pgarachne-macos-arm64-app.zip`
- `pgarachne-macos-universal-app.zip`

## Adding a New JSON-RPC Function

1. Create the function in the `api` schema (or any schema returned by
   `pgarachne.allowed_schemas()`).
2. Accept a single `jsonb` parameter and return `json`.
3. Add a comment block with a `--- PARAMS ---` section so it surfaces in
   `pgarachne.capabilities()` output and the OpenAPI generator.
4. `GRANT EXECUTE` to the appropriate role(s).

```sql
CREATE OR REPLACE FUNCTION api.echo(payload jsonb)
RETURNS json
LANGUAGE sql
AS $$
  SELECT payload;
$$;

COMMENT ON FUNCTION api.echo(jsonb) IS 'Returns the input payload unchanged.
--- PARAMS ---
{"value": {"type": "string", "description": "Any value to echo back"}}';

GRANT EXECUTE ON FUNCTION api.echo(jsonb) TO app_user;
```

No Go changes are needed — the function is reachable as
`api.echo` once it is visible to the authenticated role.

## Adding a New MCP Method Backed by SQL

1. Implement the SQL function in `sql/mcp_functions.sql` (or, for core JSON-RPC
   machinery, in `sql/schema.sql`). The function must accept `jsonb` and
   return `json`.
2. Grant `EXECUTE` to the appropriate role(s).
3. Register the method name and the SQL function in
   `internal/server/mcp.go:37` (`mcpDatabaseMethods`).

```go
var mcpDatabaseMethods = map[string]string{
    // ...
    "completion/complete": "pgarachne.mcp_complete",
}
```

No other Go changes are required — the generic `handleMCPDatabaseMethod`
handler takes care of auth, transaction, and response shape. Methods that
need bespoke logic (such as `tools/call`) live in their own handler.

## Tools in `tools/`

- `tools/pgarachne-explorer/` — interactive JSON-RPC explorer (PWA, dark/light
  theme, offline-capable). Served as static files when `STATIC_FILES_PATH`
  is configured.
- `tools/test-sse/` — minimal HTML page for manually exercising the SSE
  endpoint from a browser.

## Database Setup for Local Development

```bash
# 1. Apply the core schema (creates pgarachne schema, tokens, capabilities,
#    idempotency, OpenAPI generator).
psql -d mydb -f sql/schema.sql

# 2. Apply MCP extensions (resources/*, prompts/*, prompts table).
psql -d mydb -f sql/mcp_functions.sql

# 3. (Optional) Load the documented example function.
psql -d mydb -f sql/seed_data.sql
```

The service user `DB_USER` must be a member of every role that will be
impersonated via `SET LOCAL ROLE` and of `pgarachne_admin` if it is going to
mint API tokens.

## Testing

- `make tests` — spins up Docker Postgres, applies both SQL files plus
  `sql/seed_data.sql`, and runs `go test ./...`. This is the canonical way to
  run the test suite.
- Protocol-level MCP tests run without a DB.
- Integration tests skip unless `PGARACHNE_TEST_DB=1` is set.
