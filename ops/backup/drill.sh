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
command -v python3 >/dev/null 2>&1 || { echo "Python 3 is required" >&2; exit 2; }

BACKUP_PATH="$(CDPATH= cd -- "$BACKUP_PATH" && pwd)"
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
MEDIA_TMP=""
MEDIA_ROWS=""
NAME="chat-dr-drill-$$"
DB="chat_restore_drill"
USER="chatdrill"
PASSWORD="drill-$$-${RANDOM:-0}"
STARTED="$(date +%s)"

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  if [ -n "$MEDIA_TMP" ]; then rm -rf -- "$MEDIA_TMP"; fi
  if [ -n "$MEDIA_ROWS" ]; then rm -f -- "$MEDIA_ROWS"; fi
}
trap cleanup EXIT INT TERM

echo "[drill] verifying backup checksums"
(
  cd "$BACKUP_PATH"
  sha256sum -c manifest.txt.sha256
  sha256sum -c postgres.dump.sha256
  if [ -f media.tar.gz ]; then
    sha256sum -c media.tar.gz.sha256
    grep -qx 'media=media.tar.gz' manifest.txt
    command -v python3 >/dev/null 2>&1 || { echo 'Python3 is required for media verification' >&2; exit 2; }
    python3 "$SCRIPT_DIR/check_media_archive.py" media.tar.gz
  else
    grep -qx 'media=none' manifest.txt
  fi
)

echo "[drill] verifying backup recency"
python3 "$SCRIPT_DIR/check_backup_age.py" "$BACKUP_PATH/manifest.txt" --max-age "${CHAT_DRILL_MAX_BACKUP_AGE_SECONDS:-0}"

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

echo "[drill] comparing restored database media references with restored files"
MEDIA_ROWS="$(mktemp)"
docker exec "$NAME" psql -At -F '|' -v ON_ERROR_STOP=1 -U "$USER" -d "$DB" -c \
  "SELECT storage_key,byte_size,encode(sha256,'hex') FROM media_objects WHERE state='ready' AND deleted_at IS NULL ORDER BY storage_key" > "$MEDIA_ROWS"
MEDIA_FILES="not_in_backup"
if [ -f "$BACKUP_PATH/media.tar.gz" ]; then
  echo "[drill] validating real media extraction in isolation"
  MEDIA_TMP="$(mktemp -d)"
  tar --no-same-owner --no-same-permissions -xzf "$BACKUP_PATH/media.tar.gz" -C "$MEDIA_TMP"
  MEDIA_FILES="$(find "$MEDIA_TMP" -type f | wc -l | tr -d ' ')"
else
  # If any active media rows exist, omitting media from the backup is data loss.
  [ ! -s "$MEDIA_ROWS" ] || { echo "[drill] database references media, but media archive is absent" >&2; exit 1; }
  MEDIA_TMP="$(mktemp -d)"
fi
python3 "$SCRIPT_DIR/check_restored_media.py" "$MEDIA_TMP" "$MEDIA_ROWS"
FINISHED="$(date +%s)"
RTO=$((FINISHED - STARTED))
echo "[drill] PASS tables=$TABLES media_files=$MEDIA_FILES rto_seconds=$RTO image=$IMAGE"
