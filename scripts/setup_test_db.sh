#!/bin/bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Admin connection (used to create roles and database)
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_ADMIN_USER="${DB_ADMIN_USER:-postgres}"
DB_ADMIN_PASSWORD="${DB_ADMIN_PASSWORD:-}"

# Test database and roles
TEST_DB_NAME="${TEST_DB_NAME:-pgarachne_test}"
PGARACHNE_USER="${PGARACHNE_USER:-pgarachne}"
PGARACHNE_PASSWORD="${PGARACHNE_PASSWORD:-pgarachne_password}"
PGARACHNE_ADMIN_ROLE="${PGARACHNE_ADMIN_ROLE:-pgarachne_admin}"
TEST_USER="${TEST_USER:-pgarachne_test_user}"
TEST_PASSWORD="${TEST_PASSWORD:-pgarachne_test_password}"

export PGPASSWORD="$DB_ADMIN_PASSWORD"

echo "Setting up test database for PgArachne"
echo "Host: $DB_HOST:$DB_PORT"
echo "Admin user: $DB_ADMIN_USER"
echo "Test DB: $TEST_DB_NAME"
echo "Proxy user: $PGARACHNE_USER"
echo "Test user: $TEST_USER"
echo ""

if ! command -v psql &> /dev/null; then
  echo "Error: psql command not found. Please install PostgreSQL client tools."
  exit 1
fi

psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -v ON_ERROR_STOP=1 <<SQL
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${PGARACHNE_USER}') THEN
    CREATE ROLE ${PGARACHNE_USER} WITH LOGIN PASSWORD '${PGARACHNE_PASSWORD}';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${PGARACHNE_ADMIN_ROLE}') THEN
    CREATE ROLE ${PGARACHNE_ADMIN_ROLE};
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${TEST_USER}') THEN
    CREATE ROLE ${TEST_USER} WITH LOGIN PASSWORD '${TEST_PASSWORD}';
  END IF;
END
\$\$;

GRANT ${TEST_USER} TO ${PGARACHNE_USER};
GRANT ${PGARACHNE_ADMIN_ROLE} TO ${PGARACHNE_USER};
SQL

DB_EXISTS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -tAc "SELECT 1 FROM pg_database WHERE datname='${TEST_DB_NAME}'")
if [ "$DB_EXISTS" != "1" ]; then
  createdb -h "$DB_HOST" -p "$DB_PORT" -U "$DB_ADMIN_USER" -O "$PGARACHNE_USER" "$TEST_DB_NAME"
fi

export PGPASSWORD="$PGARACHNE_PASSWORD"

psql -h "$DB_HOST" -p "$DB_PORT" -U "$PGARACHNE_USER" -d "$TEST_DB_NAME" -v ON_ERROR_STOP=1 -f "$PROJECT_ROOT/sql/schema.sql"

psql -h "$DB_HOST" -p "$DB_PORT" -U "$PGARACHNE_USER" -d "$TEST_DB_NAME" -v ON_ERROR_STOP=1 <<SQL
CREATE OR REPLACE FUNCTION api.hello_world(payload jsonb)
RETURNS json
LANGUAGE sql
AS \$\$
  SELECT '"Hello World"'::json;
\$\$;

GRANT USAGE ON SCHEMA api TO ${TEST_USER};
GRANT EXECUTE ON FUNCTION api.hello_world(jsonb) TO ${TEST_USER};
SQL

cat <<EOF
Test database is ready.

Suggested environment for tests:
  export PGARACHNE_TEST_DB=1
  export TEST_DB_NAME=${TEST_DB_NAME}
  export DB_HOST=${DB_HOST}
  export DB_PORT=${DB_PORT}
  export DB_USER=${PGARACHNE_USER}
  export PGPASSWORD=${PGARACHNE_PASSWORD}
  export JWT_SECRET=change_me_for_tests

Run tests with:
  go test ./...
EOF
