#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT/web"
command -v npm >/dev/null 2>&1 || { echo "npm is required" >&2; exit 2; }
command -v node >/dev/null 2>&1 || { echo "Node.js is required" >&2; exit 2; }

echo "[sprint-28] Resolve pinned frontend dependencies from the npm registry..."
npm install --package-lock-only --ignore-scripts --no-audit --no-fund
test -s package-lock.json || { echo "npm did not create a valid lockfile" >&2; exit 1; }

node <<'NODE'
const fs = require('node:fs');
const pkg = JSON.parse(fs.readFileSync('package.json', 'utf8'));
const lock = JSON.parse(fs.readFileSync('package-lock.json', 'utf8'));
if (!lock.packages || !lock.packages[''] || lock.lockfileVersion < 2) {
  throw new Error('npm lockfile is missing a modern package root');
}
for (const category of ['dependencies', 'devDependencies']) {
  for (const [name, version] of Object.entries(pkg[category] || {})) {
    if (lock.packages[''][category]?.[name] !== version) {
      throw new Error('Lockfile does not match pinned ' + name + '@' + version);
    }
  }
}
console.log('[sprint-28] Frontend lockfile roots match package.json');
NODE

echo "[sprint-28] Reinstall exactly the locked dependencies"
npm ci --ignore-scripts --no-audit --no-fund
echo "[sprint-28] Typecheck"
npm run typecheck
echo "[sprint-28] Production build"
npm run build
echo "[sprint-28] PASS. Review and commit web/package-lock.json to main."
echo "  git add web/package-lock.json"
echo "  git commit -m 'build(web): lock frontend dependencies for reproducible releases'"
echo "  git push origin main"
