# PgArachne AI Context

## Project Summary
PgArachne is a Go-based HTTP gateway that exposes PostgreSQL functions as JSON-RPC 2.0 endpoints. It maps the JSON-RPC `method` to `schema.function(jsonb)` calls, impersonates a DB role via `SET LOCAL ROLE`, and returns JSON-RPC responses. It also provides a `capabilities` method backed by SQL introspection in `sql/schema.sql`, an SSE endpoint that forwards PostgreSQL `NOTIFY` payloads to clients, and a native MCP (Model Context Protocol) endpoint that allows AI clients to discover and call PostgreSQL functions as tools.

## Technology Stack
- Language: Go (module in `go.mod`)
- HTTP framework: Gin (`github.com/gin-gonic/gin`)
- DB driver: `lib/pq`
- Auth: JWT (`github.com/golang-jwt/jwt/v4`) + DB-backed API tokens
- JWT issue/parse helpers live in `internal/auth/jwt.go`; SHA-256 token hashing for long-lived API tokens lives in `internal/auth/auth.go`
- Observability: Prometheus metrics (`/metrics`)
- Release/build: GoReleaser + Makefile orchestration
- Docs: Hugo sources in `docs-src/`, generated static site in `docs/`; i18n strings in `docs-src/i18n/*.yaml`

## Runtime Flow (Core)
- `cmd/pgarachne/main.go` loads config, configures logging, then runs the server.
- `internal/server/server.go` registers routes and runs the HTTP server.
- Requests to `/{prefix}/:database/jsonrpc`:
  - JSON-RPC request body is parsed and `method` is required.
  - DB role is determined via JWT or API token middleware.
  - A transaction is opened, `SET LOCAL ROLE` is applied, and `schema.function(jsonb)` is called.
  - `capabilities` is handled specially and maps to `pgarachne.capabilities`.

## Authentication
- **Login method**: JSON-RPC `get_jwt` on `POST /{prefix}/:database/jsonrpc` connects to the DB using provided credentials. On success issues a JWT with `db_role` and `db_name` claims.
- **Protected calls**: `POST /{prefix}/:database/jsonrpc` accepts:
  - `Authorization: Bearer <jwt>` (validated and scoped to the database)
  - Or a long-lived API token validated by `SELECT pgarachne.verify_api_token($1)`
- Auth is identical for the MCP endpoint — `initialize` and `ping` are unauthenticated; all other MCP methods require a valid token.

## Database Schema
### sql/schema.sql
- `pgarachne.api_tokens` — stores hashed long-lived tokens.
- `pgarachne.add_api_token` / `pgarachne.verify_api_token` — mint and verify tokens.
- `pgarachne.capabilities(jsonb)` — introspects `pg_proc` to list callable functions; used by both JSON-RPC and MCP `tools/list`.
- `pgarachne.generate_openapi_spec(server_url_base, db_name)` — builds an **OpenAPI 3.1** document for the current database. Pulls the method list from `pgarachne.capabilities()` so the spec stays in sync. Per-method metadata (name, description, parameters schema) lives in the standard-compliant `x-pgarachne-methods` extension on the single JSON-RPC POST operation; OpenAPI tooling that does not understand the extension ignores it safely. `GRANT EXECUTE ... TO public` is set.
- `pgarachne.requests` + `pgarachne.save_idempotency_key(text)` — idempotency key deduplication (shared by JSON-RPC and MCP).
- `pgarachne.allowed_schemas()` — returns schemas exposed via API (default: `['api']`).

### sql/mcp_functions.sql
Additional SQL objects required by the MCP `resources/*` and `prompts/*` methods:
- `pgarachne.prompts` — table storing named prompt templates (`name`, `description`, `template`, `arguments`). `GRANT SELECT TO public` allows authenticated roles to read.
- `pgarachne.mcp_list_resources(jsonb)` — lists all tables/views the current role may SELECT from, as MCP resource descriptors (`db:///schema/table` URIs).
- `pgarachne.mcp_read_resource(jsonb)` — reads up to 100 rows from a table/view identified by a `db:///schema/table` URI; validates existence and privilege before executing the dynamic query.
- `pgarachne.mcp_list_prompts(jsonb)` — returns all rows from `pgarachne.prompts` as MCP prompt descriptors.
- `pgarachne.mcp_get_prompt(jsonb)` — returns a rendered prompt template with `{{variable}}` placeholders substituted from `params.arguments`.

### sql/seed_data.sql
Optional example function `api.server_info(params jsonb)` returning PostgreSQL server
details (`server_version`, `current_user`, `current_database`, `current_time`).
Loaded by `scripts/setup_test_db.sh` so integration tests can exercise at least one
public-schema function beyond `api.hello_world`. Load manually for demos:
`psql -d mydb -f sql/seed_data.sql`.

### sql/universal_table_access.sql
Optional PostgREST-like CRUD helpers: `api.universal_read`, `api.universal_create`,
`api.universal_update`, `api.universal_delete` — each accepts a single `jsonb`
parameter and dispatches to any `schema.table` the role has privileges on.
`universal_read` validates the `select` clause against a strict regex; `order`
goes through a basic injection check (no semicolons, quotes, parentheses,
`--`). Treat these as convenience helpers; review the SQL before exposing
them on a public endpoint.

### sql/users.sql
Optional demo seed: creates the `pgarachne` service role (if missing), a `demo`
role with password `Demo1234`, grants `demo` to `pgarachne`, and mints a single
API token for the `demo` role. **Not** loaded by the test setup — only meant
for manual / demo installs.

## Configuration
Loaded from `pgarachne.env` (current dir), `$XDG_CONFIG_HOME/pgarachne/pgarachne.env`, or `/etc/pgarachne/pgarachne.env`, or directly via environment variables.

Required:
- `DB_HOST`, `DB_PORT`, `DB_USER`, `JWT_SECRET` (minimum 32 bytes, e.g. `openssl rand -hex 32`)

Common optional:
- `HTTP_PORT` (default `8080`)
- `API_PREFIX` (default `db`) — first URL path segment for all database endpoints; gives routes like `/db/:database/jsonrpc`. Only letters, digits, hyphens and underscores allowed.
- `JWT_EXPIRY_HOURS` (default `8`)
- `ALLOWED_ORIGINS` (comma-separated; unset = cross-origin browser requests disabled, set `*` explicitly to allow any origin)
- `STATIC_FILES_PATH` (serves files via `NoRoute` fallback)
- `LOG_LEVEL` (`INFO` default), `LOG_OUTPUT` (`stdout` default)
- `DB_SSLMODE` (default `require`; set `disable` explicitly for local non-TLS PostgreSQL)
- `LOGIN_RATE_LIMIT_PER_IP` (default 5× `LOGIN_RATE_LIMIT`; per-IP login limit across all usernames, `0` disables)
- `MCP_SQL_ERROR_DETAIL` (default `false`; `true` includes raw PostgreSQL error messages in MCP tool errors — helps LLM agents self-correct, but leaks schema details to authenticated callers)
- `DIRECT_POOL_LIMIT` (default `1000`; max distinct Basic-Auth credential pools)
- `DB_SSLROOTCERT`, `DB_SSLCERT`, `DB_SSLKEY` (optional TLS cert paths)
- `MAX_REQUEST_BYTES` (default body size limit, protects against oversized request DoS)
- `TRUSTED_PROXIES` (recommended when running behind reverse proxy; affects client IP handling)
- `METRICS_ENABLED` (default `true`)
- `METRICS_LISTEN_ADDR` (default `127.0.0.1:9090`, dedicated local-only metrics listener)
- `PID_FILE` (optional daemon PID file path for `-start` / `-stop`; default to user cache dir, fallback temp dir)
- SSE settings: `SSE_MAX_CHANNELS`, `SSE_MAX_CLIENTS`, `SSE_CLIENT_BUFFER`, `SSE_SEND_TIMEOUT`, `SSE_HEARTBEAT`, `SSE_IDLE_TIMEOUT`

## HTTP Endpoints
- `GET /health` — health check
- `POST /{prefix}/:database/jsonrpc` — JSON-RPC 2.0 gateway (including `get_jwt`)
- `GET /{prefix}/:database/sse` — SSE stream for PostgreSQL `NOTIFY` channels (`channels` query param required)
- `POST /{prefix}/:database/mcp` — MCP (Model Context Protocol) Streamable HTTP endpoint
- `GET /{prefix}/:database/openapi.json` — OpenAPI 3.1 spec generated on the fly by `pgarachne.generate_openapi_spec`. Unauthenticated — the spec only describes method names, not data. Served as `application/json` for OpenAPI tooling compatibility.
- `GET /metrics` — Prometheus metrics on the dedicated metrics listener (`METRICS_LISTEN_ADDR`)

## MCP Endpoint Detail (`/{prefix}/{database}/mcp`)
Implements the MCP Streamable HTTP transport (protocol version `2024-11-05`). All communication uses a single `POST` endpoint.

### Method mapping
| MCP method              | Auth | Action |
|-------------------------|:----:|--------|
| `initialize`            | No   | Returns server name, protocol version, capabilities (tools + resources + prompts) |
| `ping`                  | No   | Returns `{}` |
| `notifications/*`       | No   | Returns HTTP 202, no body |
| `tools/list`            | Yes  | Calls `pgarachne.capabilities()` as authenticated role; maps to MCP tool descriptors |
| `tools/call`            | Yes  | Calls `schema.function(arguments::jsonb)` as authenticated role; wraps result in MCP text content block |
| `resources/list`        | Yes  | Calls `pgarachne.mcp_list_resources()` as authenticated role |
| `resources/read`        | Yes  | Calls `pgarachne.mcp_read_resource({uri})` as authenticated role |
| `prompts/list`          | Yes  | Calls `pgarachne.mcp_list_prompts()` as authenticated role |
| `prompts/get`           | Yes  | Calls `pgarachne.mcp_get_prompt({name, arguments})` as authenticated role |

### Adding new MCP methods
Register the MCP method name and its SQL backing function in the `mcpDatabaseMethods` map in `internal/server/mcp.go`. No other Go changes needed.

### Error model
- **SQL failures in `tools/call`** → tool-level error (`isError: true` in result content), NOT a JSON-RPC protocol error — per MCP spec.
- **SQL failures in `resources/*` and `prompts/*`** → JSON-RPC protocol error (`error` field) — these are infrastructure queries, not user tools.

### Idempotency
`tools/call` supports `_meta.idempotencyKey` in params, identical semantics to the JSON-RPC `idempotencyKey` field. Both use `pgarachne.save_idempotency_key()`.

## URL Structure Rationale
All endpoints follow `/{prefix}/{database}/{protocol}` (e.g. `/db/mydb/jsonrpc`, `/db/mydb/sse`, `/db/mydb/mcp`). The prefix defaults to `db` and is configurable via `API_PREFIX`. This structure enables:
- Reverse proxy routing by prefix/database name without body inspection.
- Horizontal scaling: route traffic to per-database instances via standard proxy rules.
- Per-database auth/rate-limiting at the proxy layer.
- Observability: log aggregators can filter by database from the URL alone.

## Key Files
- `cmd/pgarachne/main.go` — CLI entrypoint, logging, config, daemon flags
- `internal/server/server.go` — routes, auth, JSON-RPC execution
- `internal/server/mcp.go` — MCP Streamable HTTP handler; `mcpDatabaseMethods` map for extensibility
- `internal/server/sse.go` — SSE hub, per-database `pq.Listener`, client broadcast, metrics, `Shutdown(ctx)` for graceful termination
- `internal/database/database.go` — connection pool per database, `CloseAll()` for graceful shutdown
- `internal/server/metrics.go` — Prometheus collectors + HTTP/method-level middleware
- `internal/server/types.go` — JSON-RPC envelope types (`JSONRPCRequest`, `JSONRPCResponse`, `LoginRequest`)
- `internal/server/mcp_test.go` — MCP unit + integration tests (protocol-level tests run without DB; integration tests require `PGARACHNE_TEST_DB=1`)
- `internal/server/integration_test.go` — JSON-RPC + SSE integration tests
- `internal/server/validation_test.go` — identifier safety validation tests
- `internal/database/database.go` — connection pool per database
- `internal/config/config.go` — config loading and validation (includes `APIPrefix`)
- `internal/auth/auth.go` — token hashing helper (SHA-256)
- `internal/auth/jwt.go` — JWT issue/parse primitives (`Issue`, `Parse`, `Claims`, `ErrInvalidToken`)
- `internal/daemon/*` — unix background start/stop, Windows stubs
- `sql/schema.sql` — core DB schema: tokens, capabilities, idempotency, OpenAPI generation
- `sql/mcp_functions.sql` — MCP-specific SQL: `pgarachne.prompts` table, `mcp_list_resources`, `mcp_read_resource`, `mcp_list_prompts`, `mcp_get_prompt`
- `sql/seed_data.sql` — optional example function `api.server_info`; loaded by test setup
- `sql/universal_table_access.sql` — optional PostgREST-like CRUD helpers
- `sql/users.sql` — optional demo roles + initial API token
- `scripts/setup_test_db.sh` — applies `schema.sql`, `mcp_functions.sql`, and `seed_data.sql` to the test database
- `scripts/run_tests.sh` — spins up Docker Postgres, calls `setup_test_db.sh`, runs `go test ./...`
- `tools/pgarachne-explorer/` — interactive JSON-RPC explorer (PWA, dark/light theme, offline-capable). Served when `STATIC_FILES_PATH` points to this directory; URL is `/tools/pgarachne-explorer/` (or `/tools/api-explorer/` per Hugo docs)
- `tools/test-sse/` — minimal HTML page for manually exercising the SSE endpoint from a browser
- `docs-src/` — Hugo documentation sources (7 languages: cs, en, de, es, fr, it, pt)
- `docs-src/i18n/*.yaml` — localised hero section strings; rendered with `| safeHTML` to allow `<br>` tags
- `docs-src/layouts/index.html` — home page template; uses `| safeHTML` for i18n hero fields
- `docs/` — generated static documentation site
- `.goreleaser.yaml` — CLI release artifacts (darwin/linux/windows, amd64+arm64)
- `scripts/generate_homebrew_formula.sh` / `generate_homebrew_cask.sh` — Homebrew tap files
- `scripts/publish_release.sh` — tags, pushes, creates the GitHub release, updates the `heptau/tap` Homebrew tap; invoked by `make release`
- `Makefile` — `make build`, `make tests` (Docker + Go), `make release-local` (build+verify, no push), `make release` (release-local + publish), `make docs`

## Testing
- `make tests` — runs all tests: starts a Docker Postgres container, applies `schema.sql` + `mcp_functions.sql` + `seed_data.sql`, executes `go test ./...`, tears down.
- Protocol-level MCP tests (no DB needed) run always.
- Integration tests skip unless `PGARACHNE_TEST_DB=1` is set.
- Test helpers: `mcpDo`, `decodeMCPResponse`, `newProtocolTestServer`, `insertTestPrompt`, `deleteTestPrompt`.

## Guidance for AI Assistance
- Keep solutions aligned with the JSON-RPC + PostgreSQL function model (single `jsonb` param).
- Do not suggest bypassing DB role security; the role switching is intentional.
- When adding features, consider updates needed in both Go server and SQL files (`schema.sql` and/or `mcp_functions.sql`).
- New MCP methods backed by SQL: add a row to `mcpDatabaseMethods` in `mcp.go` + implement the function in `mcp_functions.sql`. No other Go changes needed.
- Prefer changes that preserve the minimal middleware footprint and performance.
- Keep release artifacts and docs in sync when changing file names/URLs.
- The MCP endpoint is a translation layer only — PostgreSQL functions remain JSON-RPC-shaped and know nothing about MCP.
- Documentation links in `_index.html` must use relative paths without language prefix (e.g. `architectural-decisions/`, not `cs/architectural-decisions.html`) because Hugo adds the language prefix automatically.
- i18n strings in `docs-src/i18n/*.yaml` that contain HTML (e.g. `<br>`) require the template to use `| safeHTML`; this is already applied in `docs-src/layouts/index.html`.
- For contributor-facing workflow and contribution conventions, see `CONTRIBUTING.md`.

## AI_CONTEXT Maintenance Rule
- This file should be updated automatically after meaningful architectural, security, release, routing, configuration, or documentation workflow changes.
- Do not wait for a special request when the update is clearly relevant.

## SSE Shutdown Semantics
- `sseHub.Shutdown(ctx)` drops all clients (signals `client.done`), closes every `pq.Listener`, and detaches the `dbs` map. New SSE requests issued after `Shutdown` begins see a fresh, empty hub.
- `handleSSE` includes `case <-client.done: return` so a server-driven drop is observed immediately. Without it the handler would only exit on idle timeout (default 30s) or client request cancellation.
- **Test strategy for shutdown**: assert on the server-side invariant (`sseHub.dbs` empty, `sseDropsCounter{dbName,"shutdown"}` incremented) rather than the client seeing EOF. Go's `http.Server` keeps the underlying TCP connection alive for keep-alive even after the chunked response terminator, so SSE clients will not see EOF until the idle timeout — that is by design and outside the scope of graceful shutdown.

## Local Postgres.app `trust` Auth Caveat
- Postgres.app on macOS ships with `pg_hba.conf` set to `trust` for localhost. Negative-login tests (`TestLoginInvalidCredentials`, `TestLoginRateLimit`) cannot distinguish wrong passwords on such a setup, so they are skipped via `requireEnforcedPasswordAuth` when wrong-password login is accepted. CI (Docker Postgres) uses `scram-sha-256` and runs them normally.

## Coverage & Benchmarks
- `make coverage` — runs `go test -coverprofile=coverage.out -covermode=atomic ./...`. Standalone (without `PGARACHNE_TEST_DB`) lands around 24%; with `PGARACHNE_TEST_DB=1` it climbs to ~60%. The integration suites cover `internal/server`, `internal/database`, and the SQL-backed branches of `internal/auth`.
- `make cover-html` — converts `coverage.out` to `coverage.html`. Excluded from git via `.gitignore`.
- `make bench` — runs `go test -run='^$' -bench=. -benchmem ./internal/auth ./internal/config ./internal/server`. Pure-CPU hot paths only (no DB). The Go test runner takes the `-run='^$'` filter so no test cases are executed — the bench is the only thing measured.
- The CI workflow generates `coverage.out` as part of `scripts/run_tests.sh` (the integration-test step, while the Docker Postgres container is still up — a separate later step re-running `go test` with `PGARACHNE_TEST_DB=1` would fail, since the container is torn down as soon as that script exits) and uploads it to Codecov (`continue-on-error: true` so the workflow still passes when the `CODECOV_TOKEN` secret is missing). The README badge points at `https://codecov.io/gh/heptau/pgarachne`.

## OpenAPI Spec
- `pgarachne.generate_openapi_spec(server_url_base, db_name)` is a `SECURITY DEFINER` `STABLE` plpgsql function. It pulls the method list from `pgarachne.capabilities()` (the same source the JSON-RPC `capabilities` method uses), so the spec stays in sync with `pg_proc`. The doc is **OpenAPI 3.1.0** and contains:
  - `info.title` derived from `CURRENT_CATALOG`; `info.summary` static.
  - `servers[0].url` built from `server_url_base || '/' || pgarachne.api_prefix() || '/' || CURRENT_CATALOG || '/jsonrpc'`.
  - `paths` containing one entry: the JSON-RPC POST operation. The path key uses the configured `api_prefix` so it reflects the live routing.
  - `x-pgarachne-methods` extension on the POST operation: an array of `{name, description, parameters}` for every callable method.
  - Standard response codes 200/400/401/409/429/500 with human descriptions.
  - `components.securitySchemes.BearerAuth` (HTTP bearer) for both JWT and API tokens.
- The Go route `GET /{prefix}/:database/openapi.json` calls the function with the request's `Host` header (so the spec matches the client's view of the public base URL) and serves the result as `application/json` with `Cache-Control: no-cache`. The route is intentionally unauthenticated — the spec only lists method names and descriptions, never data.

