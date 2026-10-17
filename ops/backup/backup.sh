#!/usr/bin/env sh
set -eu

umask 077

: "${DATABASE_URL:?DATABASE_URL is required}"
BACKUP_DIR="${CHAT_BACKUP_DIR:-./backups}"
MEDIA_ROOT="${CHAT_MEDIA_ROOT:-./var/media}"
ALLOW_MISSING_MEDIA="${CHAT_BACKUP_ALLOW_MISSING_MEDIA:-0}"
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

# A timestamp identifies one immutable backup. An atomic lock prevents two
# processes from producing or overwriting the same snapshot.
mkdir -p "$BACKUP_DIR"
BACKUP_DIR="$(CDPATH= cd -- "$BACKUP_DIR" && pwd -P)"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
DEST="$BACKUP_DIR/$STAMP"
LOCK="$BACKUP_DIR/.$STAMP.lock"
if ! mkdir "$LOCK" 2>/dev/null; then
  echo "Backup for timestamp $STAMP is already running" >&2
  exit 1
fi
TMP=""
cleanup() {
  if [ -n "$TMP" ] && [ -d "$TMP" ]; then rm -rf -- "$TMP"; fi
  if [ -d "$LOCK" ]; then rmdir "$LOCK"; fi
}
trap cleanup EXIT INT TERM
if [ -e "$DEST" ]; then
  echo "Refusing to overwrite existing backup: $DEST" >&2
  exit 1
fi
TMP="$(mktemp -d "$BACKUP_DIR/.$STAMP.tmp.XXXXXX")"

printf '%s\n' "Creating PostgreSQL backup..."
pg_dump --dbname="$DATABASE_URL" --format=custom --compress=6 --no-owner --no-acl --file="$TMP/postgres.dump"
pg_restore --list "$TMP/postgres.dump" >/dev/null
# Checksums MUST record portable basenames, not temporary file paths; after
# promotion the temporary directory no longer exists.
(cd "$TMP" && sha256sum postgres.dump > postgres.dump.sha256)

if [ ! -d "$MEDIA_ROOT" ] && [ "$ALLOW_MISSING_MEDIA" != "1" ]; then
  echo "Media directory is missing; refusing incomplete backup: $MEDIA_ROOT (set CHAT_BACKUP_ALLOW_MISSING_MEDIA=1 only for a verified media-free deployment)" >&2
  exit 1
fi
if [ -d "$MEDIA_ROOT" ]; then
  printf '%s\n' "Creating media backup..."
  tar -C "$MEDIA_ROOT" -czf "$TMP/media.tar.gz" .
  command -v python3 >/dev/null 2>&1 || { echo 'Python3 required for safe media backup' >&2; exit 2; }
  python3 "$SCRIPT_DIR/check_media_archive.py" "$TMP/media.tar.gz"
  (cd "$TMP" && sha256sum media.tar.gz > media.tar.gz.sha256)
fi

cat > "$TMP/manifest.txt" <<EOF
created_at=${STAMP}
format=chat-backup-v1
postgres=postgres.dump
media=$(if [ -f "$TMP/media.tar.gz" ]; then printf 'media.tar.gz'; else printf 'none'; fi)
EOF
(cd "$TMP" && sha256sum manifest.txt > manifest.txt.sha256)

# Promote atomically only after every step completes, never overwrite or nest
# a backup under an existing backup directory.
if [ -e "$DEST" ]; then
  echo "Destination appeared during backup, refusing overwrite: $DEST" >&2
  exit 1
fi
mv -T -- "$TMP" "$DEST"
TMP=""
rmdir "$LOCK"
trap - EXIT INT TERM
# A successful pg_dump is not yet a verified, relocatable backup. Fail the
# scheduled job loudly if the promoted artifact cannot be read and checked.
"$SCRIPT_DIR/verify.sh" "$DEST"
printf '%s\n' "Backup completed and verified: $DEST"
