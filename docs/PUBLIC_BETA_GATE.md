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
- active owner/security account list and passkey coverage;
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
