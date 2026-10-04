#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

fail() { printf 'RELEASE BLOCKED: %s\n' "$*" >&2; exit 1; }
info() { printf '==> %s\n' "$*"; }

info "checking migration version uniqueness"
duplicates="$(find backend/migrations -maxdepth 1 -type f -name '*.up.sql' -printf '%f\n' | sed -E 's/^([0-9]+)_.*/\1/' | sort | uniq -d)"
[[ -z "$duplicates" ]] || fail "duplicate migration versions: $duplicates"

info "checking required operational documentation"
for file in docs/MASTER_PLAN.md docs/SECURITY.md docs/SECURITY_INCIDENT_RUNBOOK.md docs/DISASTER_RECOVERY.md docs/RELEASE_CHECKLIST.md docs/CLOSED_ALPHA_RUNBOOK.md; do
  [[ -s "$file" ]] || fail "missing $file"
done

if [[ "${CHAT_SKIP_MIGRATION_VERIFY:-0}" != "1" ]]; then
  info "verifying clean PostgreSQL migration chain"
  bash ops/migrations/verify-clean.sh
else
  printf 'WARNING: clean migration verification skipped explicitly. Do not use this setting for a release record.\n' >&2
fi

info "checking production compose topology"
command -v docker >/dev/null 2>&1 || fail "docker is required for production compose validation"
[[ -s .env.production.example ]] || fail "missing .env.production.example"
docker compose --env-file .env.production.example -f docker-compose.prod.yml config >/dev/null

grep -q '^  caddy:' docker-compose.prod.yml || fail "production compose must define caddy ingress"
grep -q '"80:80"' docker-compose.prod.yml || fail "caddy HTTP ingress is missing"
grep -q '"443:443"' docker-compose.prod.yml || fail "caddy HTTPS ingress is missing"

for service in postgres redis nats api web; do
  block="$(awk -v service="$service" '
    $0 ~ "^  " service ":" {inside=1; next}
    inside && $0 ~ "^  [a-zA-Z0-9_-]+:" {exit}
    inside {print}
  ' docker-compose.prod.yml)"
  if printf '%s\n' "$block" | grep -q '^[[:space:]]*ports:'; then
    fail "$service must not publish host ports in production"
  fi
done

if command -v go >/dev/null 2>&1; then
  info "checking Go formatting"
  unformatted="$(gofmt -l backend || true)"
  [[ -z "$unformatted" ]] || fail "gofmt required:\n$unformatted"

  info "running Go vet"
  (cd backend && go vet ./...)

  info "running Go tests with race detector"
  (cd backend && go test -race ./...)
else
  fail "Go toolchain is required for a release check"
fi

if command -v npm >/dev/null 2>&1; then
  [[ -f web/package-lock.json ]] || fail "web/package-lock.json is required before production release"
  info "installing frontend dependencies from lockfile"
  (cd web && npm ci --ignore-scripts)
  info "running frontend typecheck"
  (cd web && npm run typecheck)
  info "running frontend production build"
  (cd web && npm run build)
else
  fail "npm is required for a release check"
fi

if [[ "${CHAT_ENV:-}" == "production" ]]; then
  info "checking production environment"
  [[ -n "${CHAT_DATABASE_URL:-}" ]] || fail "CHAT_DATABASE_URL is required"
  [[ "${CHAT_WEB_ORIGIN:-}" == https://* ]] || fail "CHAT_WEB_ORIGIN must use HTTPS"
  [[ "${CHAT_MAGIC_LINK_BASE_URL:-}" == https://* ]] || fail "CHAT_MAGIC_LINK_BASE_URL must use HTTPS"
  [[ -n "${CHAT_SMTP_ADDR:-}" ]] || fail "CHAT_SMTP_ADDR is required"
  [[ -n "${CHAT_TRUSTED_PROXY_CIDRS:-}" ]] || fail "CHAT_TRUSTED_PROXY_CIDRS is required"
  [[ -n "${CHAT_SMTP_FROM:-}" ]] || fail "CHAT_SMTP_FROM is required"
  [[ "${CHAT_SMTP_FROM,,}" != *example.com* ]] || fail "CHAT_SMTP_FROM still uses example.com"
fi

info "release code gates passed"
printf 'NOTE: backup restore drill, load test and independent pentest remain external release gates.\n'
