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
  - The role switch is issued as `SELECT set_config('role', $1, true)` (the function form of `SET LOCAL`), not as concatenated SQL text — keep it that way, the role name must stay a bind parameter.
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
- `pgarachne.generate_openapi_spec(server_url_base, db_name)` — builds an **OpenAPI 3.1** document for the current database, filtered to the methods the calling role may execute. Pulls the method list from `pgarachne.capabilities()` so the spec stays in sync. `paths` contains the real `/jsonrpc` operation (with its per-method metadata still also carried in the `x-pgarachne-methods` extension, for tooling that understands it) plus one virtual, documentation-only `/rpc/{method}` path per exposed method, for tooling that expects one path per operation (Swagger UI, Postman, codegen). The function is deliberately `SECURITY INVOKER` (not `DEFINER`) so that its internal call to `capabilities()` — which filters via `has_function_privilege(current_user, ...)` — sees the caller's actual role. `GRANT EXECUTE ... TO public` is set.
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
- `STATIC_FILES_PATH` (serves files via `NoRoute` fallback; the directory is pinned with `os.OpenRoot` at startup and every request is resolved through that `*os.Root`, so traversal and symlink escapes are refused by the OS — do not reintroduce `filepath.Join` + string containment checks here)
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
- `GET /{prefix}/:database/sse` — SSE stream for PostgreSQL `NOTIFY` channels (`channels` query param required). Authenticates the caller but, unlike every other endpoint here, does **not** `SET LOCAL ROLE` or check per-channel permissions — all SSE clients for a database share one `LISTEN` connection opened as `DB_USER`, so any authenticated caller can subscribe to any channel name. This is by design (Postgres channels aren't objects with their own GRANTs) and documented in `docs-src/content/en/real-time-notifications.html`; don't "fix" it locally without reading that note first.
- `POST /{prefix}/:database/mcp` — MCP (Model Context Protocol) Streamable HTTP endpoint
- `GET /{prefix}/:database/openapi.json` (or `/openapi.yaml`, or `?format=yaml`) — OpenAPI 3.1 spec generated on the fly by `pgarachne.generate_openapi_spec`. Requires the same auth as `/jsonrpc` (Basic / Bearer JWT / API token); the returned spec is filtered to the methods the authenticated role may execute, same as MCP `tools/list`. Served as `application/json` by default, or `application/yaml` for the YAML variant.
- `GET /metrics` — Prometheus metrics on the dedicated metrics listener (`METRICS_LISTEN_ADDR`)

## MCP Endpoint Detail (`/{prefix}/{database}/mcp`)
Implements the MCP Streamable HTTP transport, protocol version `2026-07-28`. This
revision made the protocol fully stateless: there is no `initialize`/session
handshake, no `Mcp-Session-Id`, and no GET/SSE stream — every request is an
independent `POST` that declares its own protocol version and capabilities.
GET and DELETE on the endpoint return `405`.

### Transport-level requirements (every request)
- `params._meta` must include `io.modelcontextprotocol/protocolVersion` and
  `io.modelcontextprotocol/clientCapabilities` → missing either is `-32602`
  (Invalid params), HTTP 400.
- HTTP headers `MCP-Protocol-Version` and `Mcp-Method` are required and must
  agree with the body's `_meta` version and `method` → disagreement is
  `-32020` (`HeaderMismatch`), HTTP 400.
- `tools/call`, `resources/read`, `prompts/get` additionally require an
  `Mcp-Name` header matching `params.name` (or `params.uri` for
  `resources/read`) → same `HeaderMismatch` error on mismatch.
- An unsupported `protocolVersion` (anything but `2026-07-28`) →
  `-32022` (`UnsupportedProtocolVersionError`), HTTP 400, `data.supported`
  lists the versions this server accepts.
- All of the above is validated centrally in `mcpValidateRequest`
  (`internal/server/mcp.go`) before method dispatch.

### Method mapping
| MCP method              | Auth | Action |
|-------------------------|:----:|--------|
| `server/discover`       | No   | Returns supported protocol versions, capabilities (tools + resources + prompts), server identity |
| `notifications/*`       | No   | Returns HTTP 202, no body |
| `tools/list`            | Yes  | Calls `pgarachne.capabilities()` as authenticated role; maps to MCP tool descriptors |
| `tools/call`            | Yes  | Calls `schema.function(arguments::jsonb)` as authenticated role; wraps result in MCP text content block |
| `resources/list`        | Yes  | Calls `pgarachne.mcp_list_resources()` as authenticated role |
| `resources/read`        | Yes  | Calls `pgarachne.mcp_read_resource({uri})` as authenticated role |
| `prompts/list`          | Yes  | Calls `pgarachne.mcp_list_prompts()` as authenticated role |
| `prompts/get`           | Yes  | Calls `pgarachne.mcp_get_prompt({name, arguments})` as authenticated role |

`initialize` and `ping` no longer exist — both were removed from the core
protocol in 2026-07-28.

### Result shape
Every result carries `resultType: "complete"` (PgArachne never needs
mid-call client input, so this is the only value it emits) and
`_meta["io.modelcontextprotocol/serverInfo"]`. `server/discover`,
`tools/list`, `resources/list`, `resources/read`, and `prompts/list` are also
`CacheableResult`s and additionally carry `ttlMs` + `cacheScope` (`"private"`
for anything gated by the authenticated role's grants, `"public"` for
`server/discover`). Built centrally by `mcpWrapResult`/`mcpWrapCacheableResult`
in `internal/server/mcp.go`.

### Adding new MCP methods
Register the MCP method name, its SQL backing function, and its cache hints
in the `mcpDatabaseMethods` map in `internal/server/mcp.go`. No other Go
changes needed.

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
- `pgarachne.generate_openapi_spec(server_url_base, db_name)` is a `SECURITY INVOKER` (no `SECURITY DEFINER` clause — deliberately, see below) `STABLE` plpgsql function. It pulls the method list from `pgarachne.capabilities()` (the same source the JSON-RPC `capabilities` method and MCP `tools/list` use), so the spec stays in sync with `pg_proc` **and** is filtered to what the calling role may execute. The doc is **OpenAPI 3.1.0** and contains:
  - `info.title` derived from `CURRENT_CATALOG`; `info.summary` static.
  - `servers[0].url` is the bare `server_url_base` (origin only) — every `paths` key is already a full absolute path, and OpenAPI 3.1 resolves an operation's URL as `server.url + path key`, so the server entry must not also carry a path prefix or URLs built from the spec would double up.
  - `paths` containing the real JSON-RPC POST operation at `/{prefix}/{database}/jsonrpc`, **plus** one virtual, documentation-only path per exposed method at `/{prefix}/{database}/rpc/{method}` (method names may contain dots, e.g. `api.hello_world` — used verbatim, no escaping needed). These `/rpc/*` paths do not exist as real HTTP routes; every actual call still goes through `/jsonrpc`, and each virtual operation's `description` spells out the real JSON-RPC request needed to invoke it. This exists because tooling that expects one operation per path (Swagger UI, Postman, codegen) can't otherwise represent N method signatures under a single JSON-RPC path.
  - `x-pgarachne-methods` extension on the `/jsonrpc` POST operation only: an array of `{name, description, parameters}` for every callable method, for tooling that understands the extension.
  - Standard response codes 200/400/401/403/404/409/429/500 with human descriptions, referencing shared `components.schemas.JsonRpcResponse` / `JsonRpcError` schemas via `$ref`.
  - `components.schemas.JsonRpcRequest` / `JsonRpcResponse` / `JsonRpcError` — reusable named schemas for the JSON-RPC envelope, referenced from both the `/jsonrpc` path and the virtual per-method paths' responses.
  - `components.securitySchemes.BearerAuth` (HTTP bearer) for both JWT and API tokens.
- The Go route `GET /{prefix}/:database/openapi.json` (`handleOpenAPISpec`/`handleOpenAPISpecFormat` in `internal/server/server.go`) requires the same three-mode auth as `/jsonrpc` (Basic / Bearer JWT / API token), opens a tx, calls `setupRequestTx` to `SET LOCAL ROLE` to the authenticated role, then calls the function with the request's `Host` header (so the spec matches the client's view of the public base URL) and serves the result as `application/json` with `Cache-Control: no-cache`. An unauthenticated request gets `401`, same as `/jsonrpc`.
- `GET /{prefix}/:database/openapi.yaml` (or `?format=yaml` on either route) returns the same document converted to YAML via `github.com/goccy/go-yaml`, served as `application/yaml`.

