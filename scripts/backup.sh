#!/usr/bin/env bash
set -euo pipefail

# Backup OwnWave Postgres database and music metadata to a timestamped archive.
# Usage: ./scripts/backup.sh [destination]

DEST_DIR=${1:-./backups}
mkdir -p "$DEST_DIR"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_NAME="ownwave_backup_$TIMESTAMP.sql"

echo "Creating database backup: $DEST_DIR/$BACKUP_NAME"
docker compose -f docker-compose.yml -f docker-compose.prod.yml exec -T db \
    pg_dump -U "${POSTGRES_USER:-ownwave}" -d "${POSTGRES_DB:-ownwave}" \
    > "$DEST_DIR/$BACKUP_NAME"

echo "Backup complete: $DEST_DIR/$BACKUP_NAME"
