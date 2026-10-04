#!/usr/bin/env sh
set -eu

BACKUP_PATH="${1:-}"
IMAGE="${CHAT_DRILL_POSTGRES_IMAGE:-postgres:16-alpine}"

if [ -z "$BACKUP_PATH" ] || [ ! -d "$BACKUP_PATH" ]; then
  echo "Usage: $0 <backup-directory>" >&2
  exit 2
fi

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 2; }
command -v sha256sum >/dev/null 2>&1 || { echo "sha256sum is required" >&2; exit 2; }

BACKUP_PATH="$(CDPATH= cd -- "$BACKUP_PATH" && pwd)"
NAME="chat-dr-drill-$$"
DB="chat_restore_drill"
USER="chatdrill"
PASSWORD="drill-$$-${RANDOM:-0}"
STARTED="$(date +%s)"

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

echo "[drill] verifying backup checksums"
(
  cd "$BACKUP_PATH"
  sha256sum -c manifest.txt.sha256
  sha256sum -c postgres.dump.sha256
  if [ -f media.tar.gz ]; then
    sha256sum -c media.tar.gz.sha256
    tar -tzf media.tar.gz >/dev/null
  fi
)

echo "[drill] starting isolated PostgreSQL"
docker run -d --rm --name "$NAME"   -e POSTGRES_DB="$DB"   -e POSTGRES_USER="$USER"   -e POSTGRES_PASSWORD="$PASSWORD"   -v "$BACKUP_PATH:/backup:ro"   "$IMAGE" >/dev/null

attempt=0
until docker exec "$NAME" pg_isready -U "$USER" -d "$DB" >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  [ "$attempt" -lt 60 ] || { echo "PostgreSQL did not become ready" >&2; exit 1; }
  sleep 1
done

echo "[drill] restoring backup"
docker exec "$NAME" pg_restore   --dbname="$DB"   --username="$USER"   --no-owner --no-acl   /backup/postgres.dump

echo "[drill] validating restored schema"
TABLES="$(docker exec "$NAME" psql -At -U "$USER" -d "$DB" -c "SELECT count(*) FROM pg_catalog.pg_tables WHERE schemaname='public';")"
[ "$TABLES" -gt 0 ] || { echo "restored database contains no public tables" >&2; exit 1; }

for table in users sessions chats messages security_events; do
  exists="$(docker exec "$NAME" psql -At -U "$USER" -d "$DB" -c "SELECT to_regclass('public.$table') IS NOT NULL;")"
  [ "$exists" = "t" ] || { echo "required table missing after restore: $table" >&2; exit 1; }
done

FINISHED="$(date +%s)"
RTO=$((FINISHED - STARTED))
echo "[drill] PASS tables=$TABLES rto_seconds=$RTO image=$IMAGE"
