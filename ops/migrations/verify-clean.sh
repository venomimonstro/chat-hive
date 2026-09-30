#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MIGRATIONS="$ROOT/backend/migrations"
IMAGE="${CHAT_MIGRATION_TEST_IMAGE:-postgres:16-alpine}"
NAME="chat-migration-check-$$"
DB="chat_migration_check"
USER="chatcheck"
PASSWORD="chatcheck-$RANDOM-$RANDOM"

fail() { printf 'MIGRATION CHECK FAILED: %s\n' "$*" >&2; exit 1; }
info() { printf '==> %s\n' "$*"; }
cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM

command -v docker >/dev/null 2>&1 || fail "docker is required"
[[ -d "$MIGRATIONS" ]] || fail "migration directory not found: $MIGRATIONS"

mapfile -t up_files < <(find "$MIGRATIONS" -maxdepth 1 -type f -name '*.up.sql' | sort)
[[ ${#up_files[@]} -gt 0 ]] || fail "no up migrations found"

versions="$(printf '%s\n' "${up_files[@]##*/}" | sed -E 's/^([0-9]+)_.*/\1/' | sort)"
duplicates="$(printf '%s\n' "$versions" | uniq -d)"
[[ -z "$duplicates" ]] || fail "duplicate migration versions: $duplicates"

info "starting isolated PostgreSQL container ($IMAGE)"
docker run -d --rm --name "$NAME" \
  -e POSTGRES_DB="$DB" \
  -e POSTGRES_USER="$USER" \
  -e POSTGRES_PASSWORD="$PASSWORD" \
  "$IMAGE" >/dev/null

for attempt in $(seq 1 60); do
  if docker exec "$NAME" pg_isready -U "$USER" -d "$DB" >/dev/null 2>&1; then
    break
  fi
  [[ "$attempt" -lt 60 ]] || fail "PostgreSQL did not become ready"
  sleep 1
done

info "applying ${#up_files[@]} up migrations"
for file in "${up_files[@]}"; do
  printf '   -> %s\n' "$(basename "$file")"
  docker exec -i "$NAME" psql -v ON_ERROR_STOP=1 -U "$USER" -d "$DB" < "$file" >/dev/null
done

# Force PostgreSQL to validate that public relations are readable after the full chain.
docker exec "$NAME" psql -v ON_ERROR_STOP=1 -U "$USER" -d "$DB" -c \
  "SELECT count(*) AS public_tables FROM pg_catalog.pg_tables WHERE schemaname='public';" >/dev/null

mapfile -t down_files < <(find "$MIGRATIONS" -maxdepth 1 -type f -name '*.down.sql' | sort -r)
[[ ${#down_files[@]} -eq ${#up_files[@]} ]] || fail "up/down migration count mismatch: up=${#up_files[@]} down=${#down_files[@]}"

info "applying down migrations in reverse order"
for file in "${down_files[@]}"; do
  printf '   <- %s\n' "$(basename "$file")"
  docker exec -i "$NAME" psql -v ON_ERROR_STOP=1 -U "$USER" -d "$DB" < "$file" >/dev/null
done

remaining="$(docker exec "$NAME" psql -At -U "$USER" -d "$DB" -c "SELECT count(*) FROM pg_catalog.pg_tables WHERE schemaname='public';")"
[[ "$remaining" == "0" ]] || fail "$remaining public table(s) remain after full rollback"

info "clean migration chain verified"
