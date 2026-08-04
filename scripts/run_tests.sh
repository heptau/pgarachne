#!/bin/bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.test.yml"

DB_HOST="localhost"
DB_ADMIN_USER="postgres"
DB_ADMIN_PASSWORD="postgres"

TEST_DB_NAME="pgarachne_test"
PGARACHNE_USER="pgarachne"
PGARACHNE_PASSWORD="pgarachne_password"
TEST_USER="pgarachne_test_user"
TEST_PASSWORD="pgarachne_test_password"

cleanup() {
  docker compose -f "$COMPOSE_FILE" down -v
}
trap cleanup EXIT

echo "==> Starting Postgres (test container)"
# --wait blocks until the healthcheck below reports "healthy" (or the
# timeout elapses), instead of us polling pg_isready ourselves. The official
# postgres image restarts once during first-time initdb, so a same-process
# pg_isready loop can catch that transient setup instance as "ready" and
# then race into the restart on the very next check; Compose's own
# healthcheck-driven wait doesn't have that failure mode.
docker compose -f "$COMPOSE_FILE" up -d --wait --wait-timeout 60

# The container publishes to a random host port (see docker-compose.test.yml)
# so this never collides with another project's Postgres container on the
# same machine. Read back whatever Docker actually assigned.
DB_PORT="$(docker compose -f "$COMPOSE_FILE" port postgres 5432 | cut -d: -f2)"
if [ -z "$DB_PORT" ]; then
  echo "Could not determine the host port Docker assigned to Postgres." >&2
  exit 1
fi
echo "==> Postgres listening on $DB_HOST:$DB_PORT"

echo "==> Preparing test database"
chmod +x "$PROJECT_ROOT/scripts/setup_test_db.sh"
DB_HOST="$DB_HOST" \
DB_PORT="$DB_PORT" \
DB_ADMIN_USER="$DB_ADMIN_USER" \
DB_ADMIN_PASSWORD="$DB_ADMIN_PASSWORD" \
TEST_DB_NAME="$TEST_DB_NAME" \
PGARACHNE_USER="$PGARACHNE_USER" \
PGARACHNE_PASSWORD="$PGARACHNE_PASSWORD" \
TEST_USER="$TEST_USER" \
TEST_PASSWORD="$TEST_PASSWORD" \
"$PROJECT_ROOT/scripts/setup_test_db.sh"

echo "==> Running tests"
export PGARACHNE_TEST_DB=1
export TEST_DB_NAME="$TEST_DB_NAME"
export DB_HOST="$DB_HOST"
export DB_PORT="$DB_PORT"
export DB_USER="$PGARACHNE_USER"
export PGPASSWORD="$PGARACHNE_PASSWORD"
export JWT_SECRET="test_secret_0123456789abcdef_0123456789abcdef"

cd "$PROJECT_ROOT"
# Coverage is generated here, while Postgres is still up (the EXIT trap tears
# it down as soon as this script exits) — a separate CI step re-running
# `go test` with PGARACHNE_TEST_DB=1 after this script exits would fail with
# connection-refused, since the container is already gone by then.
go test -race -coverprofile=coverage.out -covermode=atomic ./...
