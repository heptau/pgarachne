# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Dates are the day the corresponding Git tag was created (UTC).

## [Unreleased]

### Fixed

- Docs site: `assets/script.js` was missing the cache-busting query parameter already used on `assets/style.css`, so browsers/CDNs could keep serving a stale cached copy of the script after a deploy (e.g. from before the theme/language switcher existed), making the navbar buttons appear broken until the cache expired.

## [2.0.1] - 2026-07-01

### Security

- Upgraded the transitive `github.com/quic-go/quic-go` dependency (pulled in via `gin-gonic/gin`) from v0.57.0 to v0.59.1, fixing an HTTP/3 QPACK trailer expansion memory-exhaustion vulnerability (GHSA advisory, similar to CVE-2025-64702) that could let a malicious peer trigger excessive memory allocation on both server and client.

### Added

- Split `make release` into `make release-local` (test, build, and verify all release artifacts — no git, no push) and `make release` (runs `release-local`, then tags, pushes, creates the GitHub release from `dist/RELEASE_NOTES.md` and the built archives, and updates the `heptau/tap` Homebrew formula/cask via the GitHub API). See `scripts/publish_release.sh`.
- Docs site: manual light/dark/auto theme switcher (icon button, persisted in `localStorage`, overrides `prefers-color-scheme`) and an icon-only language switcher with per-language flags, replacing the old text "Language" button.
- Docs site: heart (Support) and star (GitHub) icon links in the navbar, next to the theme/language switcher.
- Docs site: `BreadcrumbList` JSON-LD and a visually-hidden `<h1>` on every documentation page.
- Docs site: expanded `llms.txt` with a full, linked list of documentation pages (was a two-line summary).

### Changed

- Docs site: removed the top navbar text links (Features/Installation/MCP/Architecture/Support); the menu button (hamburger) is now always visible and gives access to the same pages via the existing category list. The freed space widens the search input, which also gained a magnifying-glass icon and shorter placeholders in all 7 languages.
- Docs site: the QR-code script (`qrcodejs`) now only loads on the page that uses it (Support the Development), deferred, instead of on every page in every locale.

### Fixed

- Docs site: JSON-LD structured data (and the inline search index) was double-encoded because the templates used `safeHTML` instead of `safeJS` inside `<script>` tags, producing an escaped JSON string instead of a valid JSON object on every page, in every locale. Structured data now validates correctly.

## [2.0.0] - 2026-07-01

### Removed

- **Breaking:** Removed the legacy `POST /api/:database` and `GET /sse/:database` routes (and their `307 Temporary Redirect` to `/{prefix}/:database/...`), kept since the API prefix became configurable in 1.3.0. Update clients to call `/{prefix}/:database/jsonrpc` and `/{prefix}/:database/sse` directly (prefix defaults to `db`).
- **Breaking:** Removed the deprecated JSON-RPC `login` method alias. Use `get_jwt`, which has been the recommended name since 1.3.0.

### Security

- **Breaking:** `DB_SSLMODE` now defaults to `require` instead of `disable`. Set it explicitly to `disable` for local development against a non-TLS PostgreSQL.
- **Breaking:** `ALLOWED_ORIGINS` no longer defaults to `*`. Unset means cross-origin browser requests are disabled; set `*` explicitly to allow any origin.
- **Breaking:** `JWT_SECRET` must now be at least 32 bytes; the placeholder value from the example config is rejected at startup. Generate one with `openssl rand -hex 32`.
- Added `LOGIN_RATE_LIMIT_PER_IP` (default: 5× `LOGIN_RATE_LIMIT`) to throttle login attempts per client IP across all usernames, closing a credential-spraying/enumeration gap in the previous per-(IP, username) limiter.
- MCP tool/resource errors no longer leak raw PostgreSQL error text (table/constraint names, `RAISE` messages) by default; opt back in with `MCP_SQL_ERROR_DETAIL=true` if you rely on detailed errors for LLM self-correction.
- `DIRECT_POOL_LIMIT` makes the previously hardcoded direct-auth connection pool cap (1000) configurable.

### Fixed

- Fixed a stuck `sseHub.Shutdown`: it waited on a channel that `Close()` closes synchronously, so it never actually confirmed the per-database listener goroutines had exited. Shutdown now waits on the goroutines themselves, bounded by the caller's context.
- Fixed `GetConnection`'s dead-pool recovery path: the read-path cache lookup never re-validated a closed pool, so a dropped PostgreSQL connection could leave the server permanently unable to reconnect for a given database.
- Fixed `scripts/run_sqlfloff.sh`: an unterminated quote on the cleanup line made it fail every run; the hardcoded default input path was also replaced with a required argument.

### Changed

- Extracted the SET LOCAL `app.api_prefix` / idempotency-key check / SET LOCAL ROLE sequence, previously duplicated across the JSON-RPC and MCP handlers, into a single shared helper.
- `defer tx.Rollback()` call sites now ignore the expected `sql.ErrTxDone` after a successful commit instead of discarding all rollback errors silently.

### Added

- JWT support for `JWT_ISSUER`, `JWT_AUDIENCE`, and `JWT_LEEWAY`: issued tokens can now be bound to a specific issuer/audience, with configurable clock-skew tolerance for `exp`/`nbf`/`iat` validation — useful when integrating an external identity provider ("bring your own JWT").
- Two new standalone web tools alongside the PgArachne Explorer: a JWT Getter (`/tools/get-jwt`) for minting a token from credentials, and an SSE Tester (`/tools/test-sse`) for watching `NOTIFY` events live.
- CI now runs `golangci-lint`, `go test -race`, and `govulncheck` on every push and pull request; `make lint` and `make vulncheck` targets added for local use.
- Added unit tests for the login rate limiter, `daemon.isRunning`, SSE frame-writing helpers, and `parseLogLevel`, closing several previously untested code paths.
- Translated the updated configuration reference (new defaults and options above) into all supported languages (cs, de, es, fr, it, pt), in addition to English.
- Added `CHANGELOG.md` and `make release-notes` (extracts the current version's section for use as GitHub release notes during `make release`).

## [1.3.0] - 2026-03-19

### Added

- Full [Model Context Protocol](https://modelcontextprotocol.io) (MCP) support via a new `/{prefix}/{database}/mcp` endpoint, translating `tools/list` and `tools/call` to PostgreSQL function calls using the same authentication and role-switching logic as JSON-RPC.
- Standard MCP methods `resources/list`, `resources/read`, `prompts/list`, `prompts/get`, backed by corresponding SQL functions.
- Configurable API route prefix via `API_PREFIX` (default: `db`), giving routes like `/{prefix}/{database}/jsonrpc` and `/{prefix}/{database}/sse`.
- Automatic `307 Temporary Redirect` from the legacy `/api/:database` and `/sse/:database` paths to the new prefixed routes.
- Optional top-level `idempotencyKey` field on JSON-RPC requests. Stored via `pgarachne.save_idempotency_key()` inside the transaction; duplicates are rejected with HTTP 409 and JSON-RPC error `-32000`.
- `make tests` target running the complete test suite (unit + integration).
- Basic integration tests for the core MCP methods (`resources/*`, `prompts/*`).

### Changed

- **PgArachne Explorer** rebuilt as a modern Progressive Web App: dark/light theme support (`prefers-color-scheme`), full UI/style refresh (CSS variables, card layout), JSON result highlighting, a "Copy" button for responses, password/API-token auth tabs with colored connection status, loading states, and cleaner error handling. Default API path changed from `/api/{database}` to `/{prefix}/{database}/jsonrpc`, with a configurable prefix input and `?url=` query-parameter auto-fill.
- Internal JSON-RPC login method renamed to `get_jwt`; the old `login` name is kept as a deprecated alias with a log warning.
- Documentation reorganized: Explorer moved to `/tools/api-explorer/`, new `/tools/` landing page, new "Architectural Decisions" page, improved 404 handling (root `404.html` for GitHub Pages plus a custom `NoRoute` fallback), enhanced multilingual typography via TypoLima, refactored client-side search rendering (DOM instead of `innerHTML`).
- When `LOG_OUTPUT` points to a file, structured logs go exclusively to the file; the console only shows minimal startup/version info and the log path, with errors on stderr. Duplicate startup log lines were merged into one message.

### Added (docs)

- New `/tools/` section with cards for the API Explorer and a `/tools/macos-toolbar` teaser page.
- `SECURITY.md` with vulnerability reporting guidelines.

### Security

- Added a Subresource Integrity (SRI) hash to the `qrcode.js` CDN script used in documentation.

## [1.2.0] - 2026-03-01

### Security

- Authorization headers are now strictly validated *before* opening a database connection, preventing unauthenticated clients from exhausting the connection pool.
- Prometheus metrics are no longer exposed on the main API router; they now run on a dedicated internal listener (default: `127.0.0.1:9090`), configurable via `METRICS_ENABLED` and `METRICS_LISTEN_ADDR`.
- JSON-RPC and SSE endpoints now validate JWTs/tokens before attempting to connect to PostgreSQL.
- Added `TRUSTED_PROXIES` to explicitly define safe reverse proxies; `X-Forwarded-For` is strictly ignored when unset, preventing client-IP spoofing.
- Raw database error details are no longer leaked to clients on JSON-RPC execution failures.
- Added strict HTTP server timeouts (read header, read, idle) and metrics-write timeouts to mitigate Slowloris-style DoS.
- Hardened `SECURITY DEFINER` functions by forcing a fixed `search_path` (`pg_catalog`).

### Added

- Configurable daemon PID file via `PID_FILE` for `-start`/`-stop`, with safe cross-platform defaults (user cache dir, falling back to the temp dir).
- Official Homebrew tap and a unified GoReleaser-based build pipeline (`brew install heptau/tap/pgarachne`), including auto-generated `Formula/pgarachne.rb` and `Casks/pgarachne-app.rb` for the macOS GUI bundles.

### Changed

- HTTP request logs now go through structured `slog`; console output is quieter when `LOG_OUTPUT` writes to a file.
- Removed legacy packaging targets from the Makefile in favor of a single, `VERSION`-driven `make release`.

### Documentation

- Documentation site rebuilt to static HTML via Hugo (`docs-src/`).
- Added full-text client-side search, "Copy to clipboard" code-block buttons, breadcrumb navigation, quickstart panels, and a custom 404 page.
- New guides: bring-your-own external identity provider (JWT), Nginx proxy hardening for SSE, HTTP status code mapping, and a production validation checklist.
- Added Open Graph tags, Twitter Cards, JSON-LD structured data, and fixed `hreflang` generation for i18n.
- Added `CONTRIBUTING.md` and adopted an English-only comment policy in the core codebase.

## [1.1.0] - 2026-02-07

### Added

- Real-time PostgreSQL `NOTIFY` streaming via a new SSE endpoint: `GET /sse/<database>?channels=chan1,chan2,...`.
- Comprehensive Prometheus metrics across the request lifecycle: HTTP counters/histograms with status breakdown, authentication/login success and failure counters, per-method JSON-RPC counters and histograms (normalized to `schema.function` or `"other"`), and SSE gauges (active clients, active channels, dropped connections).

### Changed

- Unified all JSON-RPC traffic, including login, onto a single `/api/<db>` endpoint; the dedicated `/api/<db>/login` route was removed in favor of a standard JSON-RPC `login` method.
- Removed the older `/api/<db>/<function>` routing style.

### Security

- SSE channel names are normalized and properly quoted, fixing behavior with quoted identifiers.
- Added a configurable `SSE_MAX_CLIENTS` limit, a per-client send buffer with timeout, and enforced client drops on send timeout to protect against slow or malicious clients.
- Listener connections are closed automatically once no active subscriptions remain; on PostgreSQL listener disconnect/reconnect, all SSE clients are forcefully dropped for a clean re-subscription. Idle timeout resets on every heartbeat.

### Testing

- Extended the integration suite for SSE (delivery, quoted channels, max clients, timeouts) and added tests verifying all new Prometheus metrics and the unified `/api/<db>` endpoint.

**Full Changelog**: https://github.com/heptau/pgarachne/compare/v1.0.3...v1.1.0

## [1.0.3] - 2026-02-06

### Security

- JSON-RPC requests now fail if `method` in the request body doesn't match the function in the URL.
- Added a configurable request size limit (`MAX_REQUEST_BYTES`, default 2 MB) with early HTTP 413 responses.
- `add_api_token` restricted to `pgarachne_admin`; `verify_api_token` restricted to `pgarachne`; token verification is now correctly scoped to the target database.
- `pgarachne.capabilities` now lists only functions the current role can `EXECUTE`.
- CORS credentials are disabled when using wildcard origins.
- Configurable login rate limiting with periodic cleanup, plus trusted-proxy support to prevent IP spoofing.

### Added

- Docker-based integration test runner (`./scripts/run_tests.sh`) with new tests for login/JWT, API tokens, capabilities, rate limiting, request size limits, and method mismatches.
- Validation tests for database/function name patterns.
- SSL/TLS configuration options for database connections; connection-string quoting for user/password/dbname values.
- `pgarachne_admin` role is created automatically where possible, with safe fallbacks when privileges are missing.

### Documentation

- Corrected configuration examples (removed `DB_NAME`, added the required `DB_PORT`) and added the required-variable list across all language docs.
- Documented `TRUSTED_PROXIES`, rate-limit caveats, and the request size limit; clarified token-minting requirements.
- Czech documentation switched fully to formal address (vykání).

**Full Changelog**: https://github.com/heptau/pgarachne/compare/v1.0.2...v1.0.3

## [1.0.2] - 2026-01-05

### Security

- Added `db_name` claim validation to JWTs, strictly binding tokens to a specific database and preventing cross-database access with a valid token.
- Secured the universal API implementation and fixed multi-row return value handling.

### Documentation

- Added `manifest.json` for PWA support (Add to Home Screen), `robots.txt`, `sitemap.xml`, canonical URLs, Open Graph tags, and `hreflang` tags linking all 7 language variants.
- Added a Comparison section detailing architectural differences between PgArachne and PostgREST.
- Improved mobile CSS readability/padding and fixed table responsiveness (horizontal scrolling).

**Full Changelog**: https://github.com/heptau/pgarachne/compare/v1.0.1...v1.0.2

## [1.0.1] - 2025-12-29

### Security

- Updated `quic-go` and `golang.org/x/crypto` dependencies to address reported vulnerabilities.

### Documentation

- Improved the root redirect page (`index.html`): retitled from "Redirecting..." to "PgArachne - Documentation", with metadata for correct search-engine indexing.

**Full Changelog**: https://github.com/heptau/pgarachne/compare/v1.0.0...v1.0.1

## [1.0.0] - 2025-12-26

Initial public release. Detailed per-feature release notes are not available for this version; see the [source tree at this tag](https://github.com/heptau/pgarachne/tree/v1.0.0) for the starting feature set (JSON-RPC gateway, PostgreSQL role masquerading, JWT authentication).

[Unreleased]: https://github.com/heptau/pgarachne/compare/v2.0.0...HEAD
[2.0.0]: https://github.com/heptau/pgarachne/compare/v1.3.0...v2.0.0
[1.3.0]: https://github.com/heptau/pgarachne/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/heptau/pgarachne/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/heptau/pgarachne/compare/v1.0.3...v1.1.0
[1.0.3]: https://github.com/heptau/pgarachne/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/heptau/pgarachne/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/heptau/pgarachne/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/heptau/pgarachne/releases/tag/v1.0.0
