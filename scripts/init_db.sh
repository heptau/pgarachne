#!/bin/bash
set -e

# Path to the project root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Default values
DB_HOST="localhost"
DB_PORT="5432"
DB_USER="postgres"
DB_NAME="pgarachne_demo"
ENV_FILE="$PROJECT_ROOT/pgarachne.env"

# Load config from env file if it exists
if [ -f "$ENV_FILE" ]; then
    echo "Loading configuration from $ENV_FILE..."
    # Export variables from the .env file
    set -o allexport
    source "$ENV_FILE"
    set +o allexport
fi

# Override with environment variables if set (already done by source, but let's be explicit about defaults if missing)
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
# DB_NAME might not be in the env file as the app connects to various DBs, but for init we need a target.
# If the user didn't specify a generic DB name, we default to pgarachne_demo for the "example" initialization.
DB_NAME="${DB_NAME:-pgarachne_demo}"

echo "Database Initialization Script"
echo "------------------------------"
echo "Target Database: $DB_NAME"
echo "Host: $DB_HOST:$DB_PORT"
echo "User: $DB_USER"
echo ""

# Check for psql
if ! command -v psql &> /dev/null; then
    echo "Error: psql command not found. Please install PostgreSQL client tools."
    exit 1
fi

# Create database if it doesn't exist
echo "Checking if database '$DB_NAME' exists..."
if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -lqt | cut -d \| -f 1 | grep -qw "$DB_NAME"; then
    echo "Database '$DB_NAME' already exists."
else
    echo "Creating database '$DB_NAME'..."
    createdb -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$DB_NAME"
fi

echo "Applying schema..."
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$PROJECT_ROOT/sql/schema.sql"

read -p "Do you want to load seed data? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Loading seed data..."
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$PROJECT_ROOT/sql/seed_data.sql"
fi

echo "Initialization complete!"
