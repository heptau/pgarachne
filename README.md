# PgArachne

<div align="center">
  <img src="docs/assets/pgarachne-logo.jpeg" alt="PgArachne Logo" width="200"/>
  <h1>PgArachne</h1>
  <p><strong>Turn PostgreSQL into a secure API. Instantly.</strong></p>
  <p>Zero boilerplate. High performance. The middleware that maps HTTP requests directly to database functions.</p>
  <a href="#quick-start">Get Started</a> • <a href="https://www.pgarachne.com/">Read Full Documentation</a>
</div>

---

**PgArachne** is a high-performance JSON-RPC 2.0 API gateway that dynamically maps URL paths to PostgreSQL functions (access via `schema.function`). It is optimized for AI consumption with dynamic function discovery, secure authentication, and production-ready features.

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

**Option B: Build from Source**
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
# Note: Password is read from .pgpass
JWT_SECRET=change_this_to_something_secret
HTTP_PORT=8080
```

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

Use `curl` to login with the `app_user` credentials and get a JWT token:

```bash
curl -X POST http://localhost:8080/api/my_database/login \
  -H "Content-Type: application/json" \
  -d '{"login": "app_user", "password": "user_password"}'
```

Response:
```json
{"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."}
```

**3. Call the Function**

Use the token to call the `hello_world` function:

```bash
export TOKEN="YOUR_JWT_TOKEN_HERE"

curl -X POST http://localhost:8080/api/my_database/api.hello_world \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "method": "api.hello_world", "params": {}, "id": 1}'
```

Response:
```json
{"jsonrpc": "2.0", "result": "Hello World", "id": 1}
```

## Documentation

Detailed documentation is available in the [`docs/`](docs/index.html) directory, including:

*   **Configuration**: Full list of environment variables (`DB_HOST`, `JWT_SECRET`, etc.).
*   **Security**: How role masquerading and API Tokens work.
*   **Deployment**: Guides for Caddy, Nginx, and Ngrok.
*   **Error Codes**: Reference for JSON-RPC 2.0 errors.

👉 [**Read the Full Documentation**](https://www.pgarachne.com/)

## Support the Development

If PgArachne saves you time, please consider replacing your "buy me a coffee" budget with a support membership.

*   ☕ [**Support on Buy Me a Coffee**](https://buymeacoffee.com/pgarachne)
*   For Bank Transfer (USD/EUR/CZK) and Crypto details, please see the [Support section in the documentation](https://www.pgarachne.com/en/#support).

## License

**The Code (MIT)**: Free for personal and commercial use. See [LICENSE](LICENSE).

**The Brand**: The "PgArachne" name and logo are trademarks of **Zbyněk Vanžura**. Please remove branding if forking or selling a managed service.
