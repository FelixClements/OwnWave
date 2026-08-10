#!/usr/bin/env bash
set -euo pipefail

# Restore OwnWave Postgres database from a pg_dump SQL file.
# Usage: ./scripts/restore.sh <backup.sql>

if [ $# -ne 1 ]; then
    echo "Usage: $0 <backup.sql>"
    exit 1
fi

BACKUP="$1"

if [ ! -f "$BACKUP" ]; then
    echo "Backup file not found: $BACKUP"
    exit 1
fi

echo "Restoring database from $BACKUP"
docker compose -f docker-compose.yml -f docker-compose.prod.yml stop go python worker watcher

docker compose -f docker-compose.yml -f docker-compose.prod.yml exec -T db \
    psql -U "${POSTGRES_USER:-ownwave}" -d "${POSTGRES_DB:-ownwave}" < "$BACKUP"

docker compose -f docker-compose.yml -f docker-compose.prod.yml start go python worker watcher

echo "Restore complete"
