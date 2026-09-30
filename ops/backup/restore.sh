#!/usr/bin/env sh
set -eu

umask 077

BACKUP_PATH="${1:-}"
: "${RESTORE_DATABASE_URL:?RESTORE_DATABASE_URL is required and must point to the restore target}"
RESTORE_MEDIA_ROOT="${RESTORE_MEDIA_ROOT:-}"
ALLOW_RESTORE="${CHAT_ALLOW_RESTORE:-}"

if [ "$ALLOW_RESTORE" != "I_UNDERSTAND_THIS_REPLACES_TARGET_DATA" ]; then
  printf '%s\n' "Refusing restore. Set CHAT_ALLOW_RESTORE=I_UNDERSTAND_THIS_REPLACES_TARGET_DATA" >&2
  exit 3
fi
if [ -z "$BACKUP_PATH" ] || [ ! -d "$BACKUP_PATH" ]; then
  printf '%s\n' "Usage: $0 <backup-directory>" >&2
  exit 2
fi

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
"$SCRIPT_DIR/verify.sh" "$BACKUP_PATH"

printf '%s\n' "Restoring PostgreSQL target..."
pg_restore --dbname="$RESTORE_DATABASE_URL" --clean --if-exists --no-owner --no-acl "$BACKUP_PATH/postgres.dump"

if [ -n "$RESTORE_MEDIA_ROOT" ] && [ -f "$BACKUP_PATH/media.tar.gz" ]; then
  printf '%s\n' "Restoring media target..."
  mkdir -p "$RESTORE_MEDIA_ROOT"
  find "$RESTORE_MEDIA_ROOT" -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
  tar -C "$RESTORE_MEDIA_ROOT" -xzf "$BACKUP_PATH/media.tar.gz"
fi

printf '%s\n' "Restore completed. Run application smoke checks before promoting this target."
