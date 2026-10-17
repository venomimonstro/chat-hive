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

# Restore is fail-closed: a PostgreSQL error must not delete the previous
# media tree. The staging directory must also be cleaned on failure.
mkdir -p "$WORK/restore-target"
printf 'keep-old\n' > "$WORK/restore-target/old.txt"
export RESTORE_DATABASE_URL='postgres://example.invalid/restore-test'
export RESTORE_MEDIA_ROOT="$WORK/restore-target"
export CHAT_ALLOW_RESTORE='I_UNDERSTAND_THIS_REPLACES_TARGET_DATA'
if sh "$ROOT/ops/backup/restore.sh" "$BACKUP" > "$WORK/failed-restore.log" 2>&1; then
  echo 'Restore succeeded despite a deliberately failing pg_restore' >&2; exit 1
fi
[ -f "$WORK/restore-target/old.txt" ] || { echo 'Failed restore destroyed original media' >&2; exit 1; }
[ -z "$(find "$WORK" -maxdepth 1 -type d -name '.chat-restore-staged.*' -print -quit)" ] || {
  echo 'Failed restore left an unverified staging directory' >&2; exit 1;
}

# A restore into the backup directory must fail before touching any DB.
export RESTORE_MEDIA_ROOT="$BACKUP/injected-media"
if sh "$ROOT/ops/backup/restore.sh" "$BACKUP" > "$WORK/unsafe-restore.log" 2>&1; then
  echo 'Unsafe restore inside the source backup was accepted' >&2; exit 1
fi
[ -f "$BACKUP/postgres.dump" ] || { echo 'Unsafe restore damaged the backup' >&2; exit 1; }
export RESTORE_MEDIA_ROOT="$WORK/restore-target"
export CHAT_MEDIA_ROOT="$WORK/restore-target"
if sh "$ROOT/ops/backup/restore.sh" "$BACKUP" > "$WORK/live-media-restore.log" 2>&1; then
  echo 'Restore into live media root was accepted' >&2; exit 1
fi
export CHAT_MEDIA_ROOT="$WORK/media"

# A successful restore stages/extracts media then replaces the old tree.
cat > "$WORK/bin/pg_restore" <<'EOF'
#!/usr/bin/env sh
exit 0
EOF
chmod +x "$WORK/bin/pg_restore"
sh "$ROOT/ops/backup/restore.sh" "$BACKUP" > "$WORK/successful-restore.log"
[ ! -e "$WORK/restore-target/old.txt" ] || { echo 'Old media was not atomically replaced' >&2; exit 1; }
cmp "$WORK/media/image.jpg" "$WORK/restore-target/image.jpg"

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

# A media archive must not recursively include the backup being produced.
export CHAT_BACKUP_DIR="$WORK/media/nested-backups"
export CHAT_MEDIA_ROOT="$WORK/media"
if bash "$ROOT/ops/backup/backup.sh" > "$WORK/nested.log" 2>&1; then
  echo 'Nested backup destination was accepted' >&2; exit 1
fi
[ -z "$(find "$WORK/media/nested-backups" -mindepth 1 -maxdepth 1 -type d ! -name '.*' -print -quit)" ] || {
  echo 'Nested backup unexpectedly created a snapshot' >&2; exit 1;
}
ln -s "$WORK/media" "$WORK/symlink-media"
export CHAT_BACKUP_DIR="$WORK/isolated-backups"
export CHAT_MEDIA_ROOT="$WORK/symlink-media"
if bash "$ROOT/ops/backup/backup.sh" > "$WORK/symlink.log" 2>&1; then
  echo 'Symlinked media root was accepted' >&2; exit 1
fi
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
