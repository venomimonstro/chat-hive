#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT/web"

command -v npm >/dev/null 2>&1 || { echo "npm is required" >&2; exit 2; }
[[ -f package.json ]] || { echo "web/package.json is missing" >&2; exit 2; }

BEFORE="$(sha256sum package.json | awk '{print $1}')"

echo "[frontend] generating package-lock.json from exact package.json versions"
npm install --package-lock-only --ignore-scripts --no-audit --no-fund

AFTER="$(sha256sum package.json | awk '{print $1}')"
[[ "$BEFORE" == "$AFTER" ]] || { echo "package.json changed while generating lockfile" >&2; exit 1; }
[[ -s package-lock.json ]] || { echo "package-lock.json was not generated" >&2; exit 1; }

echo "[frontend] verifying clean install"
rm -rf node_modules
npm ci --ignore-scripts --no-audit --no-fund

echo "[frontend] typecheck"
npm run typecheck

echo "[frontend] production build"
npm run build

echo "[frontend] PASS — commit web/package-lock.json"
