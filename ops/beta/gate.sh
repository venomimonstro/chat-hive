#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

DB_URL="${CHAT_BETA_DATABASE_URL:-}"
PENTEST_REPORT="${CHAT_BETA_PENTEST_REPORT:-}"
LEGAL_SIGNOFF="${CHAT_BETA_LEGAL_SIGNOFF:-}"

fail(){ printf 'PUBLIC BETA BLOCKED: %s\n' "$*" >&2; exit 1; }
step(){ printf '\n==> %s\n' "$*"; }

[[ -n "$DB_URL" ]] || fail "CHAT_BETA_DATABASE_URL is required"
[[ -n "$PENTEST_REPORT" && -s "$PENTEST_REPORT" ]] || fail "CHAT_BETA_PENTEST_REPORT must point to a non-empty report"
[[ -n "$LEGAL_SIGNOFF" && -s "$LEGAL_SIGNOFF" ]] || fail "CHAT_BETA_LEGAL_SIGNOFF must point to a non-empty legal/compliance sign-off"
command -v psql >/dev/null 2>&1 || fail "psql is required"

step "1/5 full Closed Alpha technical gate"
bash ops/alpha/gate.sh

step "2/5 privileged passkey coverage"
missing_passkeys="$(psql "$DB_URL" -At -v ON_ERROR_STOP=1 -c "
  SELECT count(*)
  FROM admin_users au
  WHERE au.status='active'
    AND EXISTS (SELECT 1 FROM admin_user_roles r WHERE r.user_id=au.user_id AND r.role IN ('owner','security'))
    AND NOT EXISTS (SELECT 1 FROM passkey_credentials p WHERE p.user_id=au.user_id);
")"
[[ "$missing_passkeys" == "0" ]] || fail "$missing_passkeys owner/security account(s) have no passkey"

step "3/5 unresolved security alerts"
open_alerts="$(psql "$DB_URL" -At -v ON_ERROR_STOP=1 -c "
  SELECT count(*)
  FROM security_alerts a
  JOIN security_events e ON e.id=a.event_id
  WHERE a.status='open' AND e.severity IN ('high','critical');
")"
[[ "$open_alerts" == "0" ]] || fail "$open_alerts open high/critical security alert(s)"

step "4/5 moderation backlog sanity"
critical_cases="$(psql "$DB_URL" -At -v ON_ERROR_STOP=1 -c "
  SELECT count(*) FROM moderation_cases
  WHERE status IN ('open','reviewing') AND severity='critical';
")"
[[ "$critical_cases" == "0" ]] || fail "$critical_cases critical moderation case(s) remain open"

step "5/5 external evidence"
printf "pentest_report=%s\nlegal_signoff=%s\n" "$PENTEST_REPORT" "$LEGAL_SIGNOFF"

printf '\nPUBLIC BETA TECHNICAL/OPERATIONAL GATE PASS\n'
printf 'External documents were required and present. Final launch decision remains an owner/legal/security decision.\n'
