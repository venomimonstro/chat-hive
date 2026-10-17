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

# The production application uses CHAT_DATABASE_URL, while backup jobs use
# DATABASE_URL. Compare parsed database identity, not raw URL strings.
command -v python3 >/dev/null 2>&1 || { echo 'Python 3 required to validate restore target' >&2; exit 3; }
python3 "$SCRIPT_DIR/check_restore_target.py" || exit 3

# A backup containing media should not silently restore only the database.
# Explicitly opt in only for a deliberately isolated, database-only target.
if [ -f "$BACKUP_PATH/media.tar.gz" ] && [ -z "$RESTORE_MEDIA_ROOT" ] && [ "${CHAT_ALLOW_DATABASE_ONLY_RESTORE:-0}" != "1" ]; then
  echo 'Media archive exists but RESTORE_MEDIA_ROOT is missing; refusing partial restore' >&2
  exit 3
fi
if [ -n "$RESTORE_MEDIA_ROOT" ] && [ ! -f "$BACKUP_PATH/media.tar.gz" ]; then
  echo 'Media restore requested but backup has no media archive' >&2
  exit 3
fi

# Validate all potentially destructive targets before changing the database.
MEDIA_STAGE=""
MEDIA_OLD=""
cleanup() {
  if [ -n "$MEDIA_STAGE" ] && [ -d "$MEDIA_STAGE" ]; then rm -rf -- "$MEDIA_STAGE"; fi
  # If the final rename fails after moving aside the previous directory,
  # restore it instead of destroying the last working media snapshot.
  if [ -n "$MEDIA_OLD" ] && [ -d "$MEDIA_OLD" ]; then
    if [ ! -e "$RESTORE_MEDIA_ROOT" ]; then mv -- "$MEDIA_OLD" "$RESTORE_MEDIA_ROOT"; fi
  fi
}
trap cleanup EXIT INT TERM
if [ -n "$RESTORE_MEDIA_ROOT" ] && [ -f "$BACKUP_PATH/media.tar.gz" ]; then
  case "$RESTORE_MEDIA_ROOT" in /*) ;; *) echo 'Media restore requires an absolute path' >&2; exit 3 ;; esac
  command -v realpath >/dev/null 2>&1 || { echo 'realpath required for safe media restore' >&2; exit 3; }
  target="$(realpath -m -- "$RESTORE_MEDIA_ROOT")"
  literal_target="$(realpath -ms -- "$RESTORE_MEDIA_ROOT")"
  if [ "$target" != "$literal_target" ]; then
    echo 'Refusing media restore through a symlinked parent directory' >&2
    exit 3
  fi
  case "$target" in
    /|/tmp|/home|/root|/var|/var/lib|/etc|/usr|/mnt|/srv|/opt)
      echo 'Refusing media restore into a system/shared directory' >&2; exit 3 ;;
  esac
  [ ! -L "$RESTORE_MEDIA_ROOT" ] || { echo 'Media restore root must not be a symlink' >&2; exit 3; }
  # Unpack and validate BEFORE touching the database or live media target.
  mkdir -p "$(dirname -- "$RESTORE_MEDIA_ROOT")"
  MEDIA_STAGE="$(mktemp -d "$(dirname -- "$RESTORE_MEDIA_ROOT")/.chat-restore-staged.XXXXXX")"
  tar --no-same-owner --no-same-permissions -xzf "$BACKUP_PATH/media.tar.gz" -C "$MEDIA_STAGE"
fi

printf '%s\n' "Restoring PostgreSQL target transactionally..."
pg_restore --dbname="$RESTORE_DATABASE_URL" --clean --if-exists --no-owner --no-acl --single-transaction --exit-on-error "$BACKUP_PATH/postgres.dump"

if [ -n "$MEDIA_STAGE" ]; then
  printf '%s\n' "Promoting staged media restore..."
  if [ -e "$RESTORE_MEDIA_ROOT" ]; then
    [ -d "$RESTORE_MEDIA_ROOT" ] || { echo 'Media restore target must be a directory' >&2; exit 3; }
    MEDIA_OLD="$(mktemp -d "$(dirname -- "$RESTORE_MEDIA_ROOT")/.chat-restore-previous.XXXXXX")"
    rmdir "$MEDIA_OLD"
    mv -- "$RESTORE_MEDIA_ROOT" "$MEDIA_OLD"
  fi
  mv -- "$MEDIA_STAGE" "$RESTORE_MEDIA_ROOT"
  MEDIA_STAGE=""
  if [ -n "$MEDIA_OLD" ]; then
    rm -rf -- "$MEDIA_OLD"
    MEDIA_OLD=""
  fi
fi

printf '%s\n' "Restore completed. Run application smoke checks before promoting this target."
