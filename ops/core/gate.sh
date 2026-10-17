#!/usr/bin/env bash
set -euo pipefail
umask 077
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT"
fail(){ printf 'COMMERCIAL CORE LAUNCH BLOCKED: %s\n' "$*" >&2; exit 1; }
[[ "${CHAT_ENV:-}" == "production" ]] || fail "CHAT_ENV must be production"
[[ "${CHAT_LAUNCH_MODE:-core}" == "core" ]] || fail "Public launch must use CHAT_LAUNCH_MODE=core"
command -v git >/dev/null || fail "git required"
command -v python3 >/dev/null || fail "python3 required"
COMMIT="$(git rev-parse HEAD)"
[[ -z "$(git status --porcelain --untracked-files=no)" ]] || fail "working tree must match audited release commit"
REPORT="${CHAT_CORE_PENTEST_REPORT:-}"
LEGAL="${CHAT_CORE_LEGAL_SIGNOFF:-}"
ATTESTATION="${CHAT_CORE_EXTERNAL_ATTESTATION:-}"
[[ -n "$REPORT" && -s "$REPORT" ]] || fail "independent pentest report required"
[[ -n "$LEGAL" && -s "$LEGAL" ]] || fail "production privacy/terms/legal approval required"
[[ -n "$ATTESTATION" && -s "$ATTESTATION" ]] || fail "release-specific legal/security attestation required"
python3 ops/beta/validate_attestation.py "$ATTESTATION" "$REPORT" "$LEGAL" "$COMMIT" || fail "external evidence invalid"
echo "[1/3] Full code, dependency and clean migration checks"
bash ops/release/check.sh
echo "[2/3] Real server load and isolated backup restoration"
bash ops/alpha/gate.sh
echo "[3/3] Two-user core end-to-end smoke"
python3 ops/core/smoke.py
echo "CORE RELEASE TECHNICAL GATE PASS commit=$COMMIT"
echo "Owner must still verify real-device UX, app-store/browser compatibility and sign off operational rollback."
