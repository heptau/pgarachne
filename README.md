# PgArachne

<div align="center">
  <img src="docs/assets/pgarachne-logo.jpeg" alt="PgArachne Logo" width="200"/>
  <h1>PgArachne</h1>
  <p><strong>Turn PostgreSQL into a secure API. Instantly.</strong></p>
  <p>Zero boilerplate. High performance. The middleware that maps HTTP requests directly to database functions.</p>
  <a href="#quick-start">Get Started</a> • <a href="https://www.pgarachne.com/">Read Full Documentation</a>
</div>

---

**PgArachne** is a high-performance JSON-RPC 2.0 API gateway that dynamically maps URL paths to PostgreSQL functions in the `api` schema. It is optimized for AI consumption with dynamic function discovery, secure authentication, and production-ready features.

## Key Features

*   **🚀 Rapid Prototyping**: Stop writing boilerplate CRUD controllers. Define a SQL function, and your API endpoint is ready instantly.
*   **🏢 Production Ready**: Handles connection pooling, graceful shutdowns, and Prometheus metrics.
*   **🧠 AI & LLM Friendly**: Self-describing API via `capabilities` endpoint allows AI agents to construct valid calls with zero hallucinations.
*   **🔒 Secure**: Native PostgreSQL role masquerading and JWT authentication.

## Quick Start

### 1. Download Binaries
*   **Linux**: [x64](https://raw.githubusercontent.com/heptau/pgarachne/master/bin/pgarachne-linux-amd64) | [ARM64](https://raw.githubusercontent.com/heptau/pgarachne/master/bin/pgarachne-linux-arm64)
*   **macOS**: [ARM64 (Silicon)](https://raw.githubusercontent.com/heptau/pgarachne/master/bin/pgarachne-darwin-arm64) | [x64 (Intel)](https://raw.githubusercontent.com/heptau/pgarachne/master/bin/pgarachne-darwin-amd64)
*   **Windows**: [x64](https://raw.githubusercontent.com/heptau/pgarachne/master/bin/pgarachne-windows-amd64.exe) | [ARM64](https://raw.githubusercontent.com/heptau/pgarachne/master/bin/pgarachne-windows-arm64.exe)

### 2. Or Build from Source
```bash
git clone https://github.com/heptau/pgarachne.git
cd pgarachne
go build -o pgarachne cmd/pgarachne/main.go
```

### 3. Database Setup
Initialize the schema required for API tokens:
```bash
psql -d my_database -f sql/schema.sql
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
