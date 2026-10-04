#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

ALPHA_URL="${CHAT_ALPHA_URL:-}"
ACCESS_TOKEN="${CHAT_ALPHA_ACCESS_TOKEN:-}"
CHAT_ID="${CHAT_ALPHA_CHAT_ID:-}"
BACKUP_DIR="${CHAT_ALPHA_BACKUP_DIR:-}"
WS_CONCURRENCY="${CHAT_ALPHA_WS_CONCURRENCY:-100}"
RECONNECT_WORKERS="${CHAT_ALPHA_RECONNECT_WORKERS:-25}"
RECONNECT_ROUNDS="${CHAT_ALPHA_RECONNECT_ROUNDS:-5}"
MESSAGE_REQUESTS="${CHAT_ALPHA_MESSAGE_REQUESTS:-200}"
MESSAGE_CONCURRENCY="${CHAT_ALPHA_MESSAGE_CONCURRENCY:-20}"
RECORD_DIR="${CHAT_ALPHA_RECORD_DIR:-./alpha-records}"

fail(){ printf 'ALPHA BLOCKED: %s\n' "$*" >&2; exit 1; }
step(){ printf '\n==> %s\n' "$*"; }

[[ "$ALPHA_URL" == https://* ]] || fail "CHAT_ALPHA_URL must be an https URL"
[[ -n "$ACCESS_TOKEN" ]] || fail "CHAT_ALPHA_ACCESS_TOKEN is required for the full Alpha gate"
[[ -n "$CHAT_ID" ]] || fail "CHAT_ALPHA_CHAT_ID is required for message load validation"
[[ -n "$BACKUP_DIR" && -d "$BACKUP_DIR" ]] || fail "CHAT_ALPHA_BACKUP_DIR must point to a completed backup"

command -v go >/dev/null 2>&1 || fail "Go is required"
command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v git >/dev/null 2>&1 || fail "git is required"

mkdir -p "$RECORD_DIR"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
RECORD="$RECORD_DIR/$STAMP.txt"
COMMIT="$(git rev-parse HEAD 2>/dev/null || printf 'unknown')"

exec > >(tee -a "$RECORD") 2>&1

printf 'CHAT CLOSED ALPHA GATE\n'
printf 'timestamp_utc=%s\ncommit=%s\nurl=%s\n' "$STAMP" "$COMMIT" "$ALPHA_URL"

step "1/6 repository release gate"
bash ops/release/check.sh

step "2/6 deployed HTTPS smoke"
CHAT_ALPHA_URL="$ALPHA_URL" CHAT_ALPHA_ACCESS_TOKEN="$ACCESS_TOKEN" sh ops/alpha/check.sh

step "3/6 health load"
(
  cd backend
  go run ./cmd/loadtest -mode health -base "$ALPHA_URL" -requests 1000 -concurrency 50
)

step "4/6 message write load"
(
  cd backend
  CHAT_LOAD_ACCESS_TOKEN="$ACCESS_TOKEN" CHAT_LOAD_CHAT_ID="$CHAT_ID" \
    go run ./cmd/loadtest -mode messages -base "$ALPHA_URL" \
      -requests "$MESSAGE_REQUESTS" -concurrency "$MESSAGE_CONCURRENCY"
)

step "5/6 realtime connection and reconnect validation"
(
  cd backend
  CHAT_LOAD_ACCESS_TOKEN="$ACCESS_TOKEN" \
    go run ./cmd/loadtest -mode websocket -base "$ALPHA_URL" -concurrency "$WS_CONCURRENCY"
  CHAT_LOAD_ACCESS_TOKEN="$ACCESS_TOKEN" \
    go run ./cmd/loadtest -mode reconnect -base "$ALPHA_URL" \
      -concurrency "$RECONNECT_WORKERS" -reconnect-rounds "$RECONNECT_ROUNDS"
)

step "6/6 isolated restore drill"
sh ops/backup/drill.sh "$BACKUP_DIR"

printf '\nALPHA TECHNICAL GATE PASS\nrecord=%s\n' "$RECORD"
printf 'NOTE: manual product scenarios, real-device passkey smoke and independent pentest remain mandatory before Public Beta.\n'
