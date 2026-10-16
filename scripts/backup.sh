#!/usr/bin/env bash
set -euo pipefail
umask 077

# Back up the OwnWave Postgres database to a timestamped SQL dump.
# The dump includes DROP statements so restore.sh can load it into an existing database.
# Usage: ./scripts/backup.sh [destination]

DEST_DIR=${1:-./backups}
mkdir -p "$DEST_DIR"
chmod 700 "$DEST_DIR"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_NAME="ownwave_backup_$TIMESTAMP.sql"

echo "Creating database backup: $DEST_DIR/$BACKUP_NAME"
docker compose -f docker-compose.yml -f docker-compose.prod.yml exec -T db \
    pg_dump --clean --if-exists -U "${POSTGRES_USER:-ownwave}" -d "${POSTGRES_DB:-ownwave}" \
    > "$DEST_DIR/$BACKUP_NAME"

chmod 600 "$DEST_DIR/$BACKUP_NAME"

echo "Backup complete: $DEST_DIR/$BACKUP_NAME"
