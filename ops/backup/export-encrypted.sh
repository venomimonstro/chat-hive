#!/usr/bin/env bash
set -euo pipefail
umask 077

# Produce transportable age-encrypted tar snapshot for off-site storage.
# Private decryption keys must stay on a separate recovery host.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
BACKUP="${1:?Usage: bash ops/backup/export-encrypted.sh <verified-backup-directory>}"
[[ -d "$BACKUP" ]] || { echo "Backup directory not found" >&2; exit 2; }
BACKUP="$(cd "$BACKUP" && pwd -P)"
OUT="${CHAT_ENCRYPTED_BACKUP_DIR:?CHAT_ENCRYPTED_BACKUP_DIR is required}"
RECIPIENT="${CHAT_BACKUP_AGE_RECIPIENT:?CHAT_BACKUP_AGE_RECIPIENT is required}"
[[ "$RECIPIENT" =~ ^age1[0-9a-z]{40,}$ ]] || { echo "Invalid age recipient" >&2; exit 2; }
command -v age >/dev/null || { echo "Install age encryption utility first" >&2; exit 2; }
command -v tar >/dev/null || { echo "tar required" >&2; exit 2; }
command -v sha256sum >/dev/null || { echo "sha256sum required" >&2; exit 2; }
bash "$ROOT/ops/backup/verify.sh" "$BACKUP"
mkdir -p "$OUT"
OUT="$(cd "$OUT" && pwd -P)"
case "$OUT/" in "$BACKUP/"*) echo "Encrypted export directory cannot be inside source" >&2; exit 2 ;; esac
NAME="$(basename "$BACKUP").chat-backup.tar.age"
DEST="$OUT/$NAME"
[[ ! -e "$DEST" ]] || { echo "Refusing to overwrite encrypted backup" >&2; exit 3; }
TMP="$(mktemp "$OUT/.encrypted.XXXXXXXX")"
trap 'rm -f -- "$TMP"' EXIT
tar --directory="$BACKUP" --create --file=- . | age -r "$RECIPIENT" -o "$TMP"
[[ -s "$TMP" ]] || { echo "Encrypted output is empty" >&2; exit 1; }
chmod 0600 "$TMP"
mv -n -- "$TMP" "$DEST"
[[ -s "$DEST" ]] || { echo "Encrypted output not promoted" >&2; exit 1; }
(cd "$OUT" && sha256sum "$NAME" > "$NAME.sha256")
chmod 0600 "$OUT/$NAME.sha256"
trap - EXIT
echo "Encrypted backup created: $DEST"
echo "Store .age + .sha256 off-site; verify full age decrypt + database/media restore on separate recovery host."
