#!/usr/bin/env sh
set -eu

BACKUP_PATH="${1:-}"
if [ -z "$BACKUP_PATH" ] || [ ! -d "$BACKUP_PATH" ]; then
  printf '%s\n' "Usage: $0 <backup-directory>" >&2
  exit 2
fi

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
BACKUP_PATH="$(CDPATH= cd -- "$BACKUP_PATH" && pwd)"
cd "$BACKUP_PATH"
sha256sum -c manifest.txt.sha256
sha256sum -c postgres.dump.sha256
pg_restore --list postgres.dump >/dev/null

if [ -f media.tar.gz ]; then
  sha256sum -c media.tar.gz.sha256
  grep -qx 'media=media.tar.gz' manifest.txt || { echo 'manifest/media mismatch' >&2; exit 1; }
  command -v python3 >/dev/null 2>&1 || { echo 'Python3 is required for safe media verification' >&2; exit 2; }
  python3 "$SCRIPT_DIR/check_media_archive.py" media.tar.gz
else
  grep -qx 'media=none' manifest.txt || { echo 'manifest requires missing media' >&2; exit 1; }
fi

printf '%s\n' "Backup integrity verified: $BACKUP_PATH"
