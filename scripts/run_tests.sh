#!/bin/bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.test.yml"

DB_HOST="localhost"
DB_PORT="54329"
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
docker compose -f "$COMPOSE_FILE" up -d

echo "==> Waiting for Postgres to be ready"
for i in {1..30}; do
  if docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U postgres >/dev/null 2>&1; then
  echo "Postgres did not become ready in time."
  exit 1
fi

echo "==> Preparing test database"
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
export JWT_SECRET="test_secret"

cd "$PROJECT_ROOT"
go test ./...
