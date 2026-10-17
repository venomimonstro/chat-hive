#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
bash -n "$ROOT/ops/backup/install-timer.sh"
# The installer must be explicit, privilege-gated and must never silently
# enable a schedule when executed without administrative privileges.
if [[ "$(id -u)" != 0 ]]; then
  if bash "$ROOT/ops/backup/install-timer.sh" > /dev/null 2>&1; then
    echo "Backup timer installer ran without root permission" >&2
    exit 1
  fi
fi
echo "[backup-timer] PASS shell syntax and explicit installation guard"
