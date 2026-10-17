# CHAT Public Beta Gate

Public Beta is not opened because the code "looks ready". It requires evidence from engineering, operations, security and legal/compliance.

## Automated gate

Run only from a controlled deployment host:

```sh
CHAT_ALPHA_URL=https://chat.example.ru \
CHAT_ALPHA_ACCESS_TOKEN='<temporary alpha test token>' \
CHAT_ALPHA_CHAT_ID='<dedicated load-test chat id>' \
CHAT_ALPHA_BACKUP_DIR='./backups/<timestamp>' \
CHAT_BETA_DATABASE_URL='<isolated/admin database connection>' \
CHAT_BETA_PENTEST_REPORT='./evidence/pentest.pdf' \
CHAT_BETA_LEGAL_SIGNOFF='./evidence/legal-signoff.txt' \
bash ops/beta/gate.sh
```

Do not commit any token, database URL or external report.

The gate performs the full Closed Alpha technical gate and additionally blocks Beta when:

- an active `owner` or `security` account has no passkey;
- privileged Admin operations are not protected by passkey-authenticated session step-up;
- an open high/critical security alert exists;
- an open/reviewing critical moderation case exists;
- the independent pentest evidence file is missing;
- legal/compliance sign-off evidence is missing.

## External gates that code cannot replace

The following are deliberately not auto-approved by the repository:

1. independent penetration test and remediation review;
2. legal/compliance review for the actual production operating model and jurisdiction;
3. production infrastructure/provider review;
4. owner approval of launch cohort and rollback owner;
5. observed Closed Alpha reliability, moderation workload and support readiness.

## Launch evidence record

Store outside the public repository:

- release commit SHA;
- release-gate output;
- Alpha gate output;
- target host specifications;
- HTTP and WebSocket load results;
- restore-drill output and measured RTO/RPO;
- pentest report version and remediation status;
- legal sign-off version/date;
- active owner/security account list, passkey coverage and confirmation that sensitive Admin access was tested from both email and passkey sessions;
- known non-blocking issues;
- rollback owner and communication plan.

## Product gate

Public Beta should not be opened solely because infrastructure is stable. Review the owner Product dashboard for:

- signup → onboarding conversion;
- signup → first-message conversion;
- D1/D7 retention;
- active users;
- follow/content creation;
- invite conversion;
- moderation backlog.

Sprint 29 exists to improve the measured weak points before widening Beta.

## Mandatory measured product iteration (Sprint 29)

Capture two owner-only `GET /api/v1/admin/product/metrics?days=30` JSON snapshots
from **real, different Alpha deployments**. Do not put auth tokens or raw
identifiers in Git. Use `ops/product/compare.py` with a full Git commit SHA
for each deployment, a specific description of the change and at least 30
eligible users per baseline/follow-up D1 or D7 cohort. Example:

```bash
python3 ops/product/compare.py evidence/before.json evidence/after.json \\
  --metric d7 --commit-before '<40-char-commit>' --commit-after '<40-char-commit>' \\
  --change 'Simplified signup to first-conversation onboarding' \\
  --out evidence/product-iteration.json
```

Set `CHAT_BETA_PRODUCT_ITERATION_RECORD=evidence/product-iteration.json` for
`ops/beta/gate.sh`. The gate refuses fabricated zero-sized cohorts,
negative measured outcomes or missing comparison records. This is an
observational product gate, **not causal evidence**. The owner must inspect
cohort composition, sample sizes and moderation workload separately.

### Evidence integrity check

For Beta, also set `CHAT_BETA_METRICS_BASELINE` and
`CHAT_BETA_METRICS_FOLLOWUP` to the two **original** JSON snapshots.
The gate recomputes the comparison and SHA-256 hashes; merely editing
`product-iteration.json` to say `improved` cannot satisfy the gate.
An acknowledged high/critical security alert also remains a blocker until
resolved, not merely acknowledged.

### Capturing source metrics securely

Use a valid owner passkey session token as an environment variable, not as a
command-line argument. Snapshots are created with mode 0600 and cannot overwrite
an earlier baseline. They contain only aggregated metrics (no user identifiers).

```bash
export CHAT_ALPHA_URL=https://<deployed-chat-host>
export CHAT_PRODUCT_ACCESS_TOKEN='<temporary owner passkey access token>'
python3 ops/product/capture.py --days 30 --out evidence/baseline.json
# After shipping/measuring the actual change:
python3 ops/product/capture.py --days 30 --out evidence/followup.json
unset CHAT_PRODUCT_ACCESS_TOKEN
```

Store snapshots and comparison records off Git and in access-controlled storage.

### External attestation record (mandatory)

Provide `CHAT_BETA_EXTERNAL_ATTESTATION` as the path to a private JSON file
that names the exact release commit, SHA-256 fingerprints of the independent
pentest and legal documents, their dated named approvers and explicit
`decision: approved`. The pentest section must include
`critical_open: 0` and `high_open: 0`. Format:

```json
{
  "schema": "chat-beta-attestation-v1",
  "release_commit": "<40-digit release SHA>",
  "pentest": {
    "sha256": "<actual SHA256>", "decision": "approved",
    "approved_by": "<independent security firm>", "approved_at": "<ISO 8601 UTC>",
    "critical_open": 0, "high_open": 0
  },
  "legal": {
    "sha256": "<actual SHA256>", "decision": "approved",
    "approved_by": "<responsible legal reviewer>", "approved_at": "<ISO 8601 UTC>"
  }
}
```

The automated gate checks document hashes, current commit, reviewer fields,
explicit outcomes and review recency. **It cannot verify cryptographic
signatures, independence, or the substantive accuracy of a report**; the
actual independent reviews and owner sign-off are still non-negotiable.
