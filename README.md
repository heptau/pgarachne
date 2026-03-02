# PgArachne

<div align="center">
  <img src="docs-src/static/assets/pgarachne-logo.jpeg" alt="PgArachne Logo" width="200"/>
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
# Optional TLS settings (default sslmode=disable)
DB_SSLMODE=disable
# DB_SSLROOTCERT=/path/to/ca.pem
# DB_SSLCERT=/path/to/client-cert.pem
# DB_SSLKEY=/path/to/client-key.pem
# Optional login rate limiting (default: 5 attempts per 1m, set 0 to disable)
LOGIN_RATE_LIMIT=5
LOGIN_RATE_WINDOW=1m
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
JWT_SECRET=change_this_to_something_secret
HTTP_PORT=8080
```

Required variables: `DB_HOST`, `DB_PORT`, `DB_USER`, `JWT_SECRET`.

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

Use the JSON-RPC `login` method to obtain a JWT token:

```bash
curl -X POST http://localhost:8080/api/my_database \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"login","params":{"login":"app_user","password":"user_password"},"id":1}'
```

Response:
```json
{"jsonrpc":"2.0","result":{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."},"id":1}
```

**3. Call the Function**

Use the token to call the `hello_world` function:
All JSON-RPC calls go to `/api/<database>` and specify the method in the JSON body.

```bash
export TOKEN="YOUR_JWT_TOKEN_HERE"

curl -X POST http://localhost:8080/api/my_database \
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
curl -N "http://localhost:8080/sse/my_database?channels=orders,users" \
  -H "Authorization: Bearer $TOKEN"
```

Each notification is delivered as JSON:

```json
{"channel":"orders","data":{"id":123,"status":"created"}}
```

If the payload is plain text, it is wrapped as a string in `data`.

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

Build release artifacts and Brew files with GoReleaser:
```bash
make release
```

This generates local release assets in `dist/`:
* CLI archives: `darwin`, `linux`, `windows` (`amd64` + `arm64`)
* macOS GUI app archives: `pgarachne-macos-amd64-app.zip`, `pgarachne-macos-arm64-app.zip`, `pgarachne-macos-universal-app.zip`

It also generates local Homebrew files for manual copy to your tap repository:
* `dist/homebrew-tap/Formula/pgarachne.rb`
* `dist/homebrew-tap/Casks/pgarachne-app.rb`

Generated documentation is available in the [`docs/`](docs/index.html) directory, including:

*   **Configuration**: Full list of environment variables (`DB_HOST`, `JWT_SECRET`, etc.).
*   **Security**: How role masquerading and API Tokens work.
*   **Deployment**: Guides for Caddy, Nginx, and Ngrok.
*   **Architectural Decisions**: Why JSON-RPC, SSE, Go, and PostgreSQL functions.
*   **Error Codes**: Reference for JSON-RPC 2.0 errors.

👉 [**Read the Full Documentation**](https://www.pgarachne.com/)

## Support the Development

If PgArachne saves you time, please consider replacing your "buy me a coffee" budget with a support membership.

*   ☕ [**Support on Buy Me a Coffee**](https://buymeacoffee.com/pgarachne)
*   For Bank Transfer (USD/EUR/CZK) and Crypto details, please see the [Support section in the documentation](https://www.pgarachne.com/#support-donate).

## License

**The Code (MIT)**: Free for personal and commercial use. See [LICENSE](LICENSE).

**The Brand**: The "PgArachne" name and logo are trademarks of **Zbyněk Vanžura**. Please remove branding if forking or selling a managed service.
