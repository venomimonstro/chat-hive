#!/usr/bin/env sh
set -eu

BACKUP_PATH="${1:-}"
if [ -z "$BACKUP_PATH" ] || [ ! -d "$BACKUP_PATH" ]; then
  printf '%s\n' "Usage: $0 <backup-directory>" >&2
  exit 2
fi

cd "$BACKUP_PATH"
sha256sum -c manifest.txt.sha256
sha256sum -c postgres.dump.sha256
pg_restore --list postgres.dump >/dev/null

if [ -f media.tar.gz ]; then
  sha256sum -c media.tar.gz.sha256
  tar -tzf media.tar.gz >/dev/null
fi

printf '%s\n' "Backup integrity verified: $BACKUP_PATH"
