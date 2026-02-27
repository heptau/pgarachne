# Contributing to PgArachne

Thanks for contributing.

## Development Basics
- Follow the project architecture: JSON-RPC over `POST /api/:database` mapped to PostgreSQL functions with single `jsonb` input.
- Keep security model intact (`SET LOCAL ROLE` and DB role-based access are intentional).
- When changing behavior, verify whether updates are needed in both Go code and SQL (`sql/schema.sql`).

## Formatting and Style
- Basic formatting rules are defined in `.editorconfig`.
- All code comments must be in English.
- Exception: comments in localized files may be in English or in the corresponding local language.

## Project Conventions
- Unified JSON-RPC endpoint is `POST /api/:database`; `method` in JSON-RPC body is required.
- SSE endpoint is `GET /sse/:database` with required `channels=...` query.
- `make release` is the primary local release entrypoint.
- Homebrew tap artifacts are generated into `dist/homebrew-tap/`.
- Docs are authored in `docs-src/` and built into `docs/`.

## Release Naming (macOS GUI ZIPs)
- `pgarachne-macos-amd64-app.zip`
- `pgarachne-macos-arm64-app.zip`
- `pgarachne-macos-universal-app.zip`
