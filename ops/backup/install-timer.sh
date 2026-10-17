#!/usr/bin/env bash
set -euo pipefail

# One-time explicit opt-in on an Ubuntu/systemd server. Never installs the
# timer as a side effect of deploy, upgrade, or application startup.
if (( EUID != 0 )); then echo "Run as root to install the backup timer" >&2; exit 2; fi
command -v systemctl >/dev/null || { echo "systemd is required" >&2; exit 2; }

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
ENV_FILE="${CHAT_BACKUP_ENV_FILE:-/etc/chat-hive/backup.env}"
# Reject newlines, whitespace, quotes and systemd specifiers in generated
# unit values; service paths are intentionally not shell-interpreted.
for value in "$ROOT" "$ENV_FILE"; do
  if [[ ! "$value" =~ ^/[A-Za-z0-9_./-]+$ ]]; then
    echo "Unsafe or non-absolute systemd path: $value" >&2; exit 2
  fi
done
[[ -f "$ENV_FILE" && ! -L "$ENV_FILE" ]] || { echo "Missing or symlinked backup env file: $ENV_FILE" >&2; exit 2; }
[[ "$(stat -c '%a' "$ENV_FILE")" =~ ^(600|400)$ ]] || {
  echo "Backup env file must have mode 0600 or 0400" >&2; exit 2;
}
# This timer executes a repository script as root. Refuse a checkout or
# credential file controlled by another OS account (and writable parents).
[[ "$(stat -c '%u' "$ENV_FILE")" == "0" ]] || {
  echo "Backup env file must be owned by root" >&2; exit 2;
}
for executable in "$ROOT" "$ROOT/ops" "$ROOT/ops/backup" "$ROOT/ops/backup/backup.sh" "$ROOT/ops/backup/verify.sh"; do
  [[ -e "$executable" && ! -L "$executable" ]] || {
    echo "Backup executable path is missing or a symlink: $executable" >&2; exit 2;
  }
  [[ "$(stat -c '%u' "$executable")" == "0" ]] || {
    echo "Root-run backup checkout must be root-owned: $executable" >&2; exit 2;
  }
  permission="$(stat -c '%a' "$executable")"
  (( (8#$permission & 8#22) == 0 )) || {
    echo "Root-run backup checkout must not be group/world writable: $executable" >&2; exit 2;
  }
done
grep -q '^DATABASE_URL=' "$ENV_FILE" || { echo "DATABASE_URL is required in $ENV_FILE" >&2; exit 2; }
grep -q '^CHAT_BACKUP_DIR=/' "$ENV_FILE" || { echo "CHAT_BACKUP_DIR must be absolute in $ENV_FILE" >&2; exit 2; }

cat > /etc/systemd/system/chat-hive-backup.service <<UNIT
[Unit]
Description=CHAT Hive verified PostgreSQL and media backup
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
EnvironmentFile=$ENV_FILE
WorkingDirectory=$ROOT
ExecStart=/bin/sh $ROOT/ops/backup/backup.sh
TimeoutStartSec=7200
UMask=0077
NoNewPrivileges=true
UNIT
cat > /etc/systemd/system/chat-hive-backup.timer <<'UNIT'
[Unit]
Description=CHAT Hive daily verified backup timer

[Timer]
OnCalendar=*-*-* 02:00:00 UTC
RandomizedDelaySec=15m
Persistent=true
AccuracySec=1m
Unit=chat-hive-backup.service

[Install]
WantedBy=timers.target
UNIT

systemctl daemon-reload
systemctl enable --now chat-hive-backup.timer
systemctl list-timers --all chat-hive-backup.timer
echo "Installed daily timer. Run systemctl start chat-hive-backup.service for an immediate, supervised backup."
