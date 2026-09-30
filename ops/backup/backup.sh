#!/usr/bin/env sh
set -eu

umask 077

: "${DATABASE_URL:?DATABASE_URL is required}"
BACKUP_DIR="${CHAT_BACKUP_DIR:-./backups}"
MEDIA_ROOT="${CHAT_MEDIA_ROOT:-./var/media}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
DEST="${BACKUP_DIR}/${STAMP}"
TMP="${DEST}.tmp"

mkdir -p "$BACKUP_DIR"
rm -rf "$TMP"
mkdir -p "$TMP"

cleanup() {
  if [ -d "$TMP" ]; then rm -rf "$TMP"; fi
}
trap cleanup EXIT INT TERM

printf '%s\n' "Creating PostgreSQL backup..."
pg_dump --dbname="$DATABASE_URL" --format=custom --compress=6 --no-owner --no-acl --file="$TMP/postgres.dump"
pg_restore --list "$TMP/postgres.dump" >/dev/null
sha256sum "$TMP/postgres.dump" > "$TMP/postgres.dump.sha256"

if [ -d "$MEDIA_ROOT" ]; then
  printf '%s\n' "Creating media backup..."
  tar -C "$MEDIA_ROOT" -czf "$TMP/media.tar.gz" .
  sha256sum "$TMP/media.tar.gz" > "$TMP/media.tar.gz.sha256"
fi

cat > "$TMP/manifest.txt" <<EOF
created_at=${STAMP}
format=chat-backup-v1
postgres=postgres.dump
media=$(if [ -f "$TMP/media.tar.gz" ]; then printf 'media.tar.gz'; else printf 'none'; fi)
EOF
sha256sum "$TMP/manifest.txt" > "$TMP/manifest.txt.sha256"

mv "$TMP" "$DEST"
trap - EXIT INT TERM
printf '%s\n' "Backup completed: $DEST"
