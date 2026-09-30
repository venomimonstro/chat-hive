# CHAT — MASTER PLAN

This file is the single source of truth for product and engineering delivery.

## Product invariant

CHAT is a mobile-first social messenger. Private communication is the primary experience; discovery, profiles, communities and creator growth exist to create and strengthen human connections rather than replace messaging with an endless content portal.

Permanent constraints:

1. Four primary navigation areas maximum: Chats, Discover, Create, Me.
2. Messaging must continue working when Feed/Discovery/analytics are degraded.
3. No political/religious profiling of users.
4. Public content and private communication have different moderation/access policies.
5. Security, object permissions, rate limits, abuse cases and auditability are part of feature DoD.
6. No production secrets or production user data are exposed to AI development agents.
7. Database changes require reversible migrations.
8. Working modules are extended, not silently replaced or duplicated.
9. PostgreSQL is the durable source of truth; realtime is an acceleration layer only.
10. New functionality may not make the four-tab product shell more complex without a proven user need.

## Architecture

- Backend: Go modular monolith.
- Durable data: PostgreSQL.
- Realtime: WebSocket with one-time tickets + PostgreSQL LISTEN/NOTIFY first; boundary remains replaceable by NATS for multi-node fanout.
- Frontend: Next.js + TypeScript mobile-first PWA.
- Offline: versioned IndexedDB outbox + REST recovery sync.
- Cache/presence: Redis reserved for ephemeral state and future distributed limits/presence.
- Async/event bus: NATS JetStream reserved for worker/event scale-out.
- Media: controlled media service now; production target remains S3-compatible object storage + CDN.
- Deployment: Docker first; no Kubernetes until measured load requires it.

## Delivery mode

Development proceeds directly in `main` at owner request. GitHub Actions/CI are intentionally not used. This does **not** remove quality gates: before any production/alpha deployment the repository must pass local/server-side Go formatting/vet/tests, frontend typecheck/build, clean-database migrations, security tests and documented release gates.

## Sprint status

| Sprint | Scope | Status |
|---|---|---|
| 00 | Product/architecture/security source of truth | DONE |
| 01 | Repository, local infrastructure, backend/frontend skeleton | DONE |
| 02 | Design system and responsive application shell | DONE |
| 03 | Identity: email magic-link, session rotation/revocation, Yandex ID, SMTP, passkeys | IN PROGRESS |
| 04 | Onboarding: username, profile, interests | DONE |
| 05 | Profiles and social graph | IN PROGRESS |
| 06 | Realtime gateway | IN PROGRESS |
| 07 | Direct messaging core | IN PROGRESS |
| 08 | Reliable messaging/offline outbox/idempotency | DONE |
| 09 | Secure image/media pipeline | DONE |
| 10 | Groups | IN PROGRESS |
| 11 | Posts: thought/photo/post | DONE |
| 12 | Feed | DONE |
| 13 | Discovery | DONE |
| 14 | New-creator exploration distribution | DONE |
| 15 | Communities | DONE |
| 16 | Channels | DONE |
| 17 | Global search | DONE |
| 18 | Stranger requests + anti-spam | DONE |
| 19 | Moderation core | DONE |
| 20 | Admin moderation console + RBAC | DONE |
| 21 | Security plane/detection | IN PROGRESS |
| 22 | Trust center / active sessions | DONE |
| 23 | Offline + low-data modes | DONE |
| 24 | Public web + invitation growth loops | IN PROGRESS |
| 25 | Performance/load profiling | IN PROGRESS |
| 26 | Security hardening + independent pentest gate | IN PROGRESS |
| 27 | Backup/disaster recovery validation | IN PROGRESS |
| 28 | Closed alpha | PLANNED |
| 29 | Product iteration from measured activation/retention | PLANNED |
| 30 | Public beta gate | PLANNED |

## Implemented product surface

### Identity / account security

Implemented: persistent email magic links, hashed one-time challenges, short-lived access credentials, HttpOnly refresh credentials, server-side sessions, refresh rotation/replay detection, session revoke/listing, production SMTP/STARTTLS, Yandex ID Authorization Code + PKCE. Yandex OAuth tokens are not persisted in the browser or CHAT database.

Remaining before Sprint 03 can close: passkey/WebAuthn credentials, shared authenticated HTTP boundary to remove repeated Bearer parsing, security events for sensitive login/recovery/session operations.

### Profiles / social graph

Implemented: username/profile/bio/interests, public profile, edit profile, follow/unfollow, block/unblock, follower/following lists, shared-interest context, self-profile canonical routing and direct-message entry from profiles. People Discovery now recommends non-followed/non-blocked users using mutual connections and shared interests.

Remaining: explicit mutual/friend semantics only if product validation still requires a separate `friend` relationship; composite pagination for very large social lists.

### Messaging / reliability

Implemented: unique direct chats, group reuse of the same messaging core, ordered per-chat sequence, `client_message_id` idempotency, cursor history, unread/read cursor, optimistic sending, reply/edit/delete, IndexedDB durable outbox with legacy migration, retry after reconnect, responsive desktop/mobile UI and deep-link chat selection.

Realtime is implemented with short-lived one-use tickets, origin validation, compression disabled, bounded send queues, read limits, exponential reconnect+jitter and PostgreSQL LISTEN/NOTIFY fanout. REST remains authoritative recovery sync.

Reactions: single-message endpoints exist and a batch reaction endpoint aggregates up to 100 message IDs in one SQL query. Remaining Sprint 07 task: wire batch reaction state/actions into chat bubbles and verify with frontend typecheck/build.

### Stranger requests / anti-spam

Direct-chat request state is part of the same chat model (`pending/accepted/rejected`). A stranger can send only one initial request message; the recipient must accept before conversation continues. Request Inbox shows contextual trust information and supports accept/reject without duplicating chats.

### Groups

Implemented: owner/admin/member, common messaging core, create/details, members, promote/demote admin, remove member, hashed expiring/limited invites, join, revoke invite, atomic owner transfer and leave-after-transfer.

Remaining: group-specific moderator actions/rate policies beyond global API limits.

### Media / posts

Secure media pipeline has metadata/ownership/state controls, file validation, image decode/re-encode path, EXIF stripping intent, access checks and cleanup compensation for failed metadata persistence. Media does not become public merely by upload. Post-media attachment validates `ready` state and ownership transactionally.

Posts support thought/photo/post, visibility, replies, reactions, saves, soft deletion, public permalink and profile rendering. Photo composer and authenticated media rendering are implemented.

### Feed / Discovery / creator growth

Feed modes: `for-you` and `following`. Ranking is explainable and excludes blocked relationships. Signals include follows, shared interests, discussion activity, explicit feedback and exploration bonus for fresh content from smaller authors. User feedback persists as `more_like_this`, `not_interested` or `hide`; hard negative feedback removes posts from future candidate selection.

Discover now includes people recommendations. New creators receive bounded exploration bonus without fake followers/reactions or guaranteed popularity.

### Communities / channels

Communities reuse normal group chats for live conversation. Public communities enter catalog only after moderation approval. Membership and chat membership remain synchronized.

Channels use the shared posts engine for broadcast content, maintain subscribers and moderation state, and support catalog/create/profile/subscription/owner publishing flows. Moderation decisions now update the underlying public entity state rather than merely closing a case.

### Search

Global search covers people, public posts and only groups where the viewer is already a member. Private group metadata is intentionally excluded from global discovery. Communities/channels use their moderated catalogs rather than leaking private entities into search.

### Notifications / PWA

Durable notification inbox, unread counts, mark-read/all-read, push subscription storage, notification generation for messages/follows/replies/channel posts, PWA manifest and service worker are implemented. Service worker intentionally never caches authenticated `/api/*` responses. Web Push cryptographic delivery is not custom-built; it remains gated on a vetted library and VAPID configuration.

### Trust / moderation / admin

Report flow is `Report → Case → Decision → Enforcement`, not direct user-triggered bans. Admin identities/roles are separate from ordinary users. Moderation queue uses RBAC and metadata-first review; ordinary admin access does not expose arbitrary private conversations. Critical admin actions are written to DB-enforced append-only audit tables. Security-event storage exists for security-plane evolution.

### Offline / weak network

Versioned IndexedDB outbox is active with retry/idempotency. Data Saver modes are implemented: Auto, Save, Maximum. Auto reacts to browser `saveData`/2G signals. Maximum mode prevents authenticated images from downloading until the user explicitly requests them.

### Edge / security hardening

Implemented: strict CORS allowlist, secure headers, no-store for API, panic recovery, request IDs, request telemetry, bounded single-node rate limiter with Retry-After, no trust of arbitrary forwarded client IP headers, WebSocket-safe telemetry handling, non-root scratch backend Docker image and dependency-locked websocket library.

### Performance / DR

A Go load harness supports health bursts, idempotent message bursts and WebSocket connection tests and reports RPS/p50/p95/p99. Actual server capacity numbers must be measured on deployment hardware before Sprint 25 closes.

DR scripts create integrity-checked PostgreSQL custom-format backups and media archives and require an explicit separate target + destructive confirmation for restore. Sprint 27 closes only after an actual restore drill succeeds.

## Remaining release-critical work

### Sprint 03

- Passkey/WebAuthn.
- Shared auth middleware/boundary.
- Identity/security audit events.

### Sprint 05

- Decide whether a distinct `friend` relation is still necessary; do not add it merely because it existed in early concepts.
- Composite pagination for social lists when measured usage requires it.

### Sprint 06–07

- Wire realtime events into messenger refresh path and reduce REST polling only after load/reconnect validation.
- Wire batch reaction data and toggle UX into message bubbles.
- Add realtime connection/slow-client metrics.

### Sprint 10

- Group moderation actions and per-action abuse thresholds.

### Sprint 21 / 26

- Security event producers for authentication anomalies/admin anomalies/rate-limit spikes.
- Production secrets manager integration in deployment environment.
- Production network segmentation/reverse proxy configuration.
- Independent pentest is still a mandatory Public Beta gate.

### Sprint 24

- Public landing and migration/invite acquisition pages need final conversion UX and analytics.
- Creator/community migration flows need measured onboarding rather than more feature scope.

### Sprint 25

- Run health/message/WebSocket profiles on actual server.
- Record concurrency ceiling, CPU/RAM, p95/p99, reconnect-storm behavior.
- Define degradation thresholds for Feed/Discovery before Messaging.

### Sprint 27

- Run backup + restore drill on isolated target.
- Record RPO/RTO and verify media/database consistency.

## Closed Alpha gate (Sprint 28)

Closed Alpha may start only when all are true:

- Clean database migrations succeed from zero.
- Backend formatting/vet/tests pass locally/server-side.
- Frontend typecheck/build pass.
- No known Critical/High exploitable security finding remains.
- Backup restore drill succeeds.
- Realtime failure does not lose acknowledged messages.
- Request Inbox prevents unsolicited multi-message spam.
- Moderation queue and admin audit are operational.
- Actual load test establishes a safe capacity limit for one node.
- Production SMTP/Yandex configuration is verified without exposing secrets.

## Public Beta gate (Sprint 30)

In addition to Alpha requirements:

- Independent penetration test completed and blocking findings fixed.
- Production monitoring/alerting and incident runbook tested.
- Legal/compliance qualification for the production service is completed separately.
- Data retention/localization configuration is verified for the target jurisdiction.
- D1/D7 Alpha retention and activation data justify broader acquisition.
- No feature is launched merely to match a competitor; scope stays tied to communication, discovery, creator growth, trust or reliability.

## Definition of Done

A feature is not DONE until all applicable items are satisfied:

- Complete user flow including loading/empty/error states.
- Mobile and desktop behavior defined.
- Server-side authentication/object authorization.
- Input validation/output encoding.
- Rate-limit/abuse analysis.
- Retry-safe/idempotent state mutation where applicable.
- Audit/security events where required.
- Critical-path tests.
- Reversible dependency-safe migration for schema changes.
- Documentation/master-plan update.
- No known Critical/High exploitable issue introduced.

## AI development protocol

1. Read this file and relevant architecture/security/data docs before modifying a sprint.
2. Audit existing implementation before creating another module.
3. Preserve working behavior unless replacement is explicit.
4. Minimize dependencies; verify version/checksum before adding one.
5. Never invent cryptography.
6. Never weaken validation/security to make a feature appear complete.
7. Never use production credentials or production user data.
8. Keep PostgreSQL/REST recovery authoritative even when realtime is available.
9. Update this document when sprint state materially changes.

## Current next action

Finish chat reaction UI and realtime-driven messenger refresh, then complete identity security/passkeys, group moderation/rate limits and security-event producers. In parallel, run actual load/restore drills on deployment infrastructure before declaring Sprint 25/27 DONE.
