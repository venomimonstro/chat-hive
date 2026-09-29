# CHAT Security Baseline

## Principles

- Defense in depth.
- Least privilege.
- Assume breach.
- Object-level authorization on every protected resource.
- No production secrets/user data available to AI agents.
- Admin/control plane separated logically from public application traffic.
- Security events are auditable and privileged access is case/role constrained.

## Required controls

Identity/session:
- short-lived access tokens;
- rotating refresh tokens stored server-side as hashes;
- reuse detection and revocation;
- passkeys mandatory for privileged accounts;
- rate limits on login/magic-link/recovery.

Application:
- strict request schemas;
- parameterized SQL;
- no raw user HTML;
- CSP/HSTS/secure cookies;
- server-side authorization for every chat/profile/admin object;
- SSRF isolation for future link previews;
- file uploads decoded/re-encoded and stored outside executable web roots.

Infrastructure:
- PostgreSQL/Redis/NATS private-only;
- non-root containers where possible;
- secrets outside Git/source;
- separate backup credentials;
- restore tests required.

Admin:
- separate privileged auth/session policy;
- RBAC;
- passkey + step-up for dangerous operations;
- dual approval for bulk exports/large message access/security policy changes;
- immutable audit trail.

Supply chain:
- pinned dependencies and lockfiles/checksums;
- SAST, secret scan, dependency/container scan in CI;
- SBOM before production releases;
- no unreviewed AI-added dependency.

## Initial threat model

P0 scenarios:
1. Super-admin/session takeover.
2. IDOR/broken access control exposing another user's chats/messages.
3. Database/backup exfiltration.
4. Stored XSS through message/post/profile content.
5. Malicious file upload.
6. SSRF through future link preview/media fetch.
7. WebSocket authorization bypass/replay/flood.
8. Supply-chain compromise of npm/Go/container/CI dependency.
9. Insider bulk export/message access.
10. DDoS/reconnect storm taking messaging offline.

## Security gate

No public beta with known exploitable Critical or unresolved High findings. Backup restore, privileged MFA/passkey, secret scanning, object-authorization tests and incident runbooks are release gates.
