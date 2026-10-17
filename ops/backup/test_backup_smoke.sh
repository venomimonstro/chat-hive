#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin" "$WORK/media" "$WORK/backups"
printf 'sample\n' > "$WORK/media/image.jpg"

# Fake only database clients so this test can run offline. Actual pg_dump
# and pg_restore compatibility is validated by the real DR drill.
cat > "$WORK/bin/pg_dump" <<'EOF'
#!/usr/bin/env bash
set -e
for arg in "$@"; do
  case "$arg" in --file=*) printf 'mock-backup\n' > "${arg#--file=}" ;; esac
done
EOF
cat > "$WORK/bin/pg_restore" <<'EOF'
#!/usr/bin/env sh
[ "$1" = '--list' ]
EOF
chmod +x "$WORK/bin/"*

export PATH="$WORK/bin:$PATH"
export DATABASE_URL='postgres://example.invalid/offline'
export CHAT_BACKUP_DIR="$WORK/backups"
export CHAT_MEDIA_ROOT="$WORK/media"
# Force the SAME timestamp for both invocations, even on slow CI/hosts.
cat > "$WORK/bin/date" <<'EOF'
#!/usr/bin/env sh
printf '20261017T120000Z\n'
EOF
chmod +x "$WORK/bin/date"
bash "$ROOT/ops/backup/backup.sh" > "$WORK/first.log"
BACKUP="$(find "$WORK/backups" -mindepth 1 -maxdepth 1 -type d ! -name '.*' -print -quit)"
[ -n "$BACKUP" ] || { echo "No completed backup" >&2; exit 1; }

# The critical regression: checksums must survive temp-dir rename.
( cd "$BACKUP" && sha256sum -c postgres.dump.sha256 &&
  sha256sum -c manifest.txt.sha256 &&
  sha256sum -c media.tar.gz.sha256 )
sh "$ROOT/ops/backup/verify.sh" "$BACKUP"

# Missing media must fail without creating a promoted backup.
mkdir -p "$WORK/no-media-backups"
export CHAT_BACKUP_DIR="$WORK/no-media-backups"
export CHAT_MEDIA_ROOT="$WORK/nonexistent-media"
unset CHAT_BACKUP_ALLOW_MISSING_MEDIA || true
if bash "$ROOT/ops/backup/backup.sh" > "$WORK/missing-media.log" 2>&1; then
  echo "Incomplete backup unexpectedly succeeded with missing media" >&2
  exit 1
fi
[ -z "$(find "$WORK/no-media-backups" -mindepth 1 -maxdepth 1 -type d ! -name '.*' -print -quit)" ] || {
  echo "Incomplete backup was promoted" >&2; exit 1;
}
export CHAT_BACKUP_DIR="$WORK/backups"
export CHAT_MEDIA_ROOT="$WORK/media"

# Two backups with the same timestamp must never overwrite the first one.
if bash "$ROOT/ops/backup/backup.sh" > "$WORK/second.log" 2>&1; then
  echo 'Backup collision unexpectedly overwrote an immutable snapshot' >&2
  exit 1
fi
[ "$(find "$WORK/backups" -mindepth 1 -maxdepth 1 -type d ! -name '.*' | wc -l)" = 1 ] || {
  echo 'Backup collision altered backup directory count' >&2; exit 1;
}
( cd "$BACKUP" && sha256sum -c postgres.dump.sha256 &&
  sha256sum -c manifest.txt.sha256 &&
  sha256sum -c media.tar.gz.sha256 )
echo '[backup-smoke] PASS: relocatable hashes, archive verification and collision protection'
