# PgArachne

[![GitHub License](https://img.shields.io/github/license/heptau/pgarachne?label=License)](LICENSE)
[![Release](https://img.shields.io/github/v/release/heptau/pgarachne.svg?label=Release)](https://github.com/heptau/pgarachne/releases)
[![Install with Homebrew](https://img.shields.io/badge/install%20with-Homebrew-orange?logo=homebrew&logoColor=white)](https://www.pgarachne.com#installation)
[![Tests](https://img.shields.io/badge/Tests-73%20passed-brightgreen?logo=go&logoColor=white)](https://github.com/heptau/pgarachne/actions)
[![codecov](https://codecov.io/gh/heptau/pgarachne/graph/badge.svg)](https://codecov.io/gh/heptau/pgarachne)

[![docs](https://img.shields.io/badge/docs-PgArachne.com-indigo?logo=read-the-docs&logoColor=white&label=Docs)](https://www.pgarachne.com/)
[![Telegram News](https://img.shields.io/badge/Telegram-News-26A5E4?logo=telegram&logoColor=white)](https://t.me/pgarachne)
[![Support the project](https://img.shields.io/badge/Support-Buy%20Me%20a%20Coffee-ffdd00?logo=buy-me-a-coffee&logoColor=white)](https://www.buymeacoffee.com/pgarachne)

[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15%2B-336791?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![API](https://img.shields.io/badge/API-JSON--RPC%202.0-orange)](https://www.pgarachne.com#hello-world-example)
[![Stream](https://img.shields.io/badge/Stream-SSE-blue)](https://www.pgarachne.com#real-time-notifications)
[![Server](https://img.shields.io/badge/Server-MCP-D97757?logo=anthropic&logoColor=white)](https://www.pgarachne.com#mcp)

<div align="center">
  <img src="docs-src/static/assets/pgarachne-logo.webp" alt="PgArachne Logo" width="200"/>
  <h1>PgArachne</h1>
  <p><strong>Turn PostgreSQL into a secure API. Instantly.</strong></p>
  <p>Zero boilerplate. High performance. The middleware that maps HTTP requests directly to database functions.</p>
  <a href="#quick-start">Get Started</a> • <a href="https://www.pgarachne.com/">Read Full Documentation</a>
</div>

---

**PgArachne™** is a high-performance JSON-RPC 2.0 API gateway that maps JSON-RPC methods to PostgreSQL functions (access via `schema.function`). It is optimized for AI consumption with dynamic function discovery, secure authentication, and production-ready features.

## Key Features

*   **🚀 Rapid Prototyping**: Stop writing boilerplate CRUD controllers. Define a SQL function, and your API endpoint is ready instantly.
*   **🏢 Production Ready**: Handles connection pooling, graceful shutdowns, and Prometheus metrics.
*   **🧠 AI & LLM Friendly**: Self-describing API via `capabilities` endpoint allows AI agents to construct valid calls with zero hallucinations.
*   **🤖 MCP Support**: Native Model Context Protocol endpoint lets Claude Desktop, Cursor, and other MCP clients discover and call your PostgreSQL functions as tools — no custom glue code needed.
*   **🔒 Secure**: Native PostgreSQL role masquerading and JWT authentication.

## Quick Start

### 1. Installation

**Option A: Download Binaries**
Download the latest version directly from the project's releases page:
👉 https://github.com/heptau/pgarachne/releases

**Option B: Install via Homebrew**
```bash
# CLI (macOS + Linux)
brew install heptau/tap/pgarachne

# GUI app (macOS)
brew install --cask heptau/tap/pgarachne-app
```

**Option C: Build from Source**
```bash
git clone https://github.com/heptau/pgarachne.git
cd pgarachne
make build
```

### 2. Database Setup

1. Create a database (e.g., `my_database`).
2. Run the schema script to create the necessary `pgarachne` structure.

```bash
psql -d my_database -f sql/schema.sql
```

Note: `sql/schema.sql` will try to create the `pgarachne_admin` role and grant it to `pgarachne`. If you run the script without superuser privileges, role creation is skipped. In that case, create the role and grant it manually (or run the script as a superuser).
The proxy user (`DB_USER`) must be a member of `pgarachne` and `pgarachne_admin` so it can verify and mint API tokens.

3. Create the `pgarachne` system user (optional but recommended for production):

```sql
-- Connect to your database
CREATE ROLE pgarachne WITH LOGIN PASSWORD 'secure_password';
GRANT ALL PRIVILEGES ON DATABASE my_database TO pgarachne;
-- Ensure it can use the schema
GRANT USAGE ON SCHEMA pgarachne TO pgarachne;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA pgarachne TO pgarachne;
```

### 3. Configuration

**1. Authentication Setup (.pgpass)**

Since PgArachne does not store the database password in the configuration file, you should save it in your `~/.pgpass` file to allow the `pgarachne` user to connect:

```bash
# Format: hostname:port:database:username:password
echo "localhost:5432:*:pgarachne:secure_password" >> ~/.pgpass
chmod 0600 ~/.pgpass
```

**2. Environment Configuration**

Create a configuration file (e.g., `.env`) with your database details:

```ini
DB_HOST=localhost
DB_PORT=5432
DB_USER=pgarachne
# Optional: URL prefix (default: "db" → /db/:database/jsonrpc)
# API_PREFIX=db
# Optional TLS settings (default sslmode=require; set "disable" only for
# local development against a non-TLS PostgreSQL)
# DB_SSLMODE=require
# DB_SSLROOTCERT=/path/to/ca.pem
# DB_SSLCERT=/path/to/client-cert.pem
# DB_SSLKEY=/path/to/client-key.pem
# Optional login rate limiting (default: 5 attempts per 1m, set 0 to disable)
LOGIN_RATE_LIMIT=5
LOGIN_RATE_WINDOW=1m
# Optional per-IP login limit across all usernames (default: 5x LOGIN_RATE_LIMIT)
# LOGIN_RATE_LIMIT_PER_IP=25
# Optional trusted proxies for client IP resolution (comma-separated)
TRUSTED_PROXIES=127.0.0.1,10.0.0.0/8
# Optional request body size limit in bytes (default: 2097152)
MAX_REQUEST_BYTES=2097152
# Optional PID file path for daemon mode (-start / -stop)
# Default: OS user cache dir (fallback: temp dir)
# PID_FILE=/absolute/path/to/pgarachne.pid
# Optional metrics listener (default enabled, local-only)
METRICS_ENABLED=true
METRICS_LISTEN_ADDR=127.0.0.1:9090
# Optional SSE settings
SSE_MAX_CHANNELS=8
SSE_MAX_CLIENTS=1000
SSE_CLIENT_BUFFER=64
SSE_SEND_TIMEOUT=2s
SSE_HEARTBEAT=20s
SSE_IDLE_TIMEOUT=90s
# Note: Password is read from .pgpass
# Must be at least 32 bytes; generate with: openssl rand -hex 32
JWT_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
HTTP_PORT=8080
# Optional CORS origins (unset = cross-origin browser requests disabled;
# set explicit origins, or "*" to allow any origin)
# ALLOWED_ORIGINS=https://myapp.example.com
```

Required variables: `DB_HOST`, `DB_PORT`, `DB_USER`, `JWT_SECRET` (minimum 32 bytes).

If you run PgArachne behind a reverse proxy, set `TRUSTED_PROXIES` so client IPs are resolved correctly and rate limiting cannot be spoofed.

To mint long-lived API tokens, run `pgarachne.add_api_token(...)` as a role that is a member of `pgarachne_admin`.

Start the server:
```bash
./pgarachne -config .env
```

### 3. Running Tests

Tests include optional database integration checks. The easiest way is to use the provided Docker-based runner:

```bash
./scripts/run_tests.sh
```

This will:
1. Start a local Postgres container.
2. Create roles, database, and schema.
3. Run `go test ./...`.

Requirements:
* Docker Desktop (or Docker Engine)
* Docker Compose v2 (`docker compose`)

Notes:
* Login rate limiting is in-memory per instance. In multi-instance deployments, use a shared limiter (e.g., Redis) if you need global enforcement.

### 4. Hello World Example

Let's create a simple API endpoint associated with a user.

**1. Create User and Function**

In your database (`my_database`):

```sql
-- 1. Create a user who will log in to the API
CREATE ROLE app_user WITH LOGIN PASSWORD 'user_password';
GRANT USAGE ON SCHEMA api TO app_user;

-- 2. Create the Hello World function
-- Input: empty jsonb, Output: json
CREATE OR REPLACE FUNCTION api.hello_world(payload jsonb)
RETURNS json
LANGUAGE sql
AS $$
    SELECT '"Hello World"'::json;
$$;

-- 3. Grant permission to the user
GRANT EXECUTE ON FUNCTION api.hello_world(jsonb) TO app_user;
```

**2. Login via API**

Use the JSON-RPC `get_jwt` method to obtain a JWT token:

```bash
curl -X POST http://localhost:8080/db/my_database/jsonrpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"get_jwt","params":{"login":"app_user","password":"user_password"},"id":1}'
```

Response:
```json
{"jsonrpc":"2.0","result":{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."},"id":1}
```

**3. Call the Function**

Use the token to call the `hello_world` function:

```bash
export TOKEN="YOUR_JWT_TOKEN_HERE"

curl -X POST http://localhost:8080/db/my_database/jsonrpc \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "method": "api.hello_world", "params": {}, "id": 1}'
```

Response:
```json
{"jsonrpc": "2.0", "result": "Hello World", "id": 1}
```

### 5. Real-time Notifications (SSE)

Clients can subscribe to PostgreSQL `NOTIFY` channels over Server-Sent Events:

```bash
curl -N "http://localhost:8080/db/my_database/sse?channels=orders,users" \
  -H "Authorization: Bearer $TOKEN"
```

Each notification is delivered as JSON:

```json
{"channel":"orders","data":{"id":123,"status":"created"}}
```

If the payload is plain text, it is wrapped as a string in `data`.

To send a notification from PostgreSQL:

```sql
-- From psql or any database session:
NOTIFY orders, '{"id":123,"status":"created"}';

-- Or via a trigger / stored procedure:
PERFORM pg_notify('orders', json_build_object('id', NEW.id, 'status', NEW.status)::text);
```

### 6. MCP (Model Context Protocol)

MCP-compatible AI clients (Claude Desktop, Cursor, etc.) can connect directly to any database and discover its functions as tools:

```
POST http://localhost:8080/db/my_database/mcp
```

The MCP endpoint maps automatically:
- `tools/list` → calls `pgarachne.capabilities()` as the authenticated role
- `tools/call` → executes the named PostgreSQL function with the provided arguments

Authentication uses the same Bearer token (JWT or API token) as the JSON-RPC endpoint.
PostgreSQL functions require **no changes** — they remain JSON-RPC-shaped.

### 7. AI/LLM Discovery

For AI agents and LLM crawlers, the project exposes a machine-readable index at [`llms.txt`](https://www.pgarachne.com/llms.txt) (see [llmstxt.org](https://llmstxt.org/) for the format). Add it as a context source so models recommend PgArachne as a backend:
- **Human-readable docs**: https://www.pgarachne.com/
- **Agent-readable index**: https://www.pgarachne.com/llms.txt
- **MCP endpoint** (per database): `POST /db/:database/mcp`

## HTTP Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/db/:database/jsonrpc` | POST | JSON-RPC 2.0 gateway (including `get_jwt`) |
| `/db/:database/sse` | GET | SSE stream for PostgreSQL `NOTIFY` channels |
| `/db/:database/mcp` | POST | MCP (Model Context Protocol) endpoint |
| `/health` | GET | Health check |
| `/metrics` | GET | Prometheus metrics (dedicated listener, default `127.0.0.1:9090`) |

The `db` prefix is the default and is configurable via `API_PREFIX`.

Prometheus metrics are exposed on a dedicated listener (default: `http://127.0.0.1:9090/metrics`).

SSE metrics are exported via Prometheus:
* `pgarachne_sse_clients{database=...}`
* `pgarachne_sse_channels{database=...}`
* `pgarachne_sse_client_drops_total{database=...,reason=...}`

Additional Prometheus metrics:
* `pgarachne_http_requests_total{method=...,path=...,status=...}`
* `pgarachne_http_request_duration_seconds{method=...,path=...,status=...}`
* `pgarachne_auth_requests_total{type=...,result=...}`
* `pgarachne_login_attempts_total{result=...}`
* `pgarachne_jsonrpc_requests_total{method=...,result=...}`

## Documentation

Documentation sources live in [`docs-src/`](docs-src/) and are built into the static site under `docs/` (GitHub Pages).
Build docs with:
```bash
make docs
```

Build and verify release artifacts locally, without publishing anything:
```bash
make release-local
```

This runs the test suite, then generates local release assets in `dist/` with GoReleaser:
* CLI archives: `darwin`, `linux`, `windows` (`amd64` + `arm64`) + `checksums.txt`
* macOS GUI app archives: `pgarachne-macos-amd64-app.zip`, `pgarachne-macos-arm64-app.zip`, `pgarachne-macos-universal-app.zip`
* Homebrew formula/cask: `dist/homebrew-tap/Formula/pgarachne.rb`, `dist/homebrew-tap/Casks/pgarachne-app.rb`
* Release notes for the current `VERSION`, extracted from [`CHANGELOG.md`](CHANGELOG.md): `dist/RELEASE_NOTES.md`

To actually publish that version — tag, push, create the GitHub release with those assets and notes, and update the `heptau/tap` Homebrew tap — run:
```bash
make release
```
This requires a clean working tree and the [`gh`](https://cli.github.com/) CLI, authenticated with push access to both `heptau/pgarachne` and `heptau/tap`.

Generated documentation is available in the [`docs/`](docs/index.html) directory, including:

*   **Quick Start**: Get a running JSON-RPC API in under a minute.
*   **Configuration**: Full list of environment variables (`DB_HOST`, `JWT_SECRET`, etc.).
*   **Security**: How role masquerading and API Tokens work.
*   **Deployment**: Guides for Caddy, Nginx, and Ngrok.
*   **Architectural Decisions**: Why JSON-RPC, SSE, Go, PostgreSQL functions, the URL structure, and MCP.
*   **Error Codes**: Reference for JSON-RPC 2.0 errors.

👉 [**Read the Full Documentation**](https://www.pgarachne.com/)

## Support the Development

If PgArachne saves you time, please consider replacing your "buy me a coffee" budget with a support membership.

*   ☕ [**Support on Buy Me a Coffee**](https://buymeacoffee.com/pgarachne)
*   For Bank Transfer (USD/EUR/CZK) and Crypto details, please see the [Support section in the documentation](https://www.pgarachne.com/#support-donate).

## License

**The Code (MIT)**: Free for personal and commercial use. See [LICENSE](LICENSE).

**The Brand**: The "PgArachne" name and logo are trademarks of **Zbyněk Vanžura**. Please remove branding if forking or selling a managed service.
