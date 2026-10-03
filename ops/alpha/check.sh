#!/bin/sh
set -eu

BASE_URL="${CHAT_ALPHA_URL:-}"
ACCESS_TOKEN="${CHAT_ALPHA_ACCESS_TOKEN:-}"

if [ -z "$BASE_URL" ]; then
  echo "CHAT_ALPHA_URL is required" >&2
  exit 2
fi

case "$BASE_URL" in
  https://*) ;;
  *) echo "CHAT_ALPHA_URL must use https" >&2; exit 2 ;;
esac

BASE_URL="${BASE_URL%/}"
tmp="${TMPDIR:-/tmp}/chat-alpha-$$"
mkdir -p "$tmp"
trap 'rm -rf "$tmp"' EXIT INT TERM

check_code() {
  expected="$1"
  path="$2"
  shift 2
  code="$(curl -sS -o "$tmp/body" -D "$tmp/headers" -w '%{http_code}' "$@" "$BASE_URL$path")"
  if [ "$code" != "$expected" ]; then
    echo "$path expected $expected, got $code" >&2
    cat "$tmp/body" >&2
    exit 1
  fi
}

echo "[alpha] health"
check_code 200 /health/live
grep -q '"status":"ok"' "$tmp/body"
check_code 200 /health/ready
grep -q '"status":"ready"' "$tmp/body"

echo "[alpha] public web headers"
check_code 200 /welcome
grep -qi '^strict-transport-security:' "$tmp/headers"
grep -qi '^x-content-type-options: nosniff' "$tmp/headers"

echo "[alpha] API boundary"
check_code 200 /api/v1
grep -qi '^cache-control: no-store' "$tmp/headers"
grep -qi '^x-frame-options: DENY' "$tmp/headers"

echo "[alpha] anonymous chat access rejected"
check_code 401 /api/v1/chats

if [ -n "$ACCESS_TOKEN" ]; then
  echo "[alpha] authenticated session"
  check_code 200 /api/v1/auth/session -H "Authorization: Bearer $ACCESS_TOKEN"
  check_code 200 /api/v1/chats -H "Authorization: Bearer $ACCESS_TOKEN"
else
  echo "[alpha] authenticated checks skipped; CHAT_ALPHA_ACCESS_TOKEN is unset"
fi

echo "[alpha] PASS"
