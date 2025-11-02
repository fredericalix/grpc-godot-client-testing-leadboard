#!/bin/bash
# dev-wait-for-db.sh - Wait for PostgreSQL to be ready

set -e

HOST="${DB_HOST:-localhost}"
PORT="${DB_PORT:-5432}"
USER="${DB_USER:-leaderboard}"
DATABASE="${DB_NAME:-leaderboard}"
MAX_RETRIES=30
RETRY_INTERVAL=1

echo "Waiting for PostgreSQL at $HOST:$PORT..."

for i in $(seq 1 $MAX_RETRIES); do
    if pg_isready -h "$HOST" -p "$PORT" -U "$USER" -d "$DATABASE" > /dev/null 2>&1; then
        echo "✓ PostgreSQL is ready!"
        exit 0
    fi

    echo "Attempt $i/$MAX_RETRIES: PostgreSQL not ready yet, waiting..."
    sleep $RETRY_INTERVAL
done

echo "✗ Error: PostgreSQL did not become ready in time"
exit 1
