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

Development proceeds directly in `main` at owner request. GitHub Actions/CI are intentionally not used. This does **not** remove quality gates: `ops/release/check.sh` is the executable local/server release gate and `docs/RELEASE_CHECKLIST.md` defines the production gate. A release still requires Go formatting/vet/race tests, frontend lockfile/typecheck/build, clean-database migrations, load validation, backup/restore validation and security review.

## Sprint status

| Sprint | Scope | Status |
|---|---|---|
| 00 | Product/architecture/security source of truth | DONE |
| 01 | Repository, local infrastructure, backend/frontend skeleton | DONE |
| 02 | Design system and responsive application shell | DONE |
| 03 | Identity: email magic-link, session rotation/revocation, Yandex ID, SMTP, passkeys | IN PROGRESS |
| 04 | Onboarding: username, profile, interests | DONE |
| 05 | Profiles and social graph | IN PROGRESS |
| 06 | Realtime gateway | DONE |
| 07 | Direct messaging core | DONE |
| 08 | Reliable messaging/offline outbox/idempotency | DONE |
| 09 | Secure image/media pipeline | DONE |
| 10 | Groups | DONE |
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
| 21 | Security plane/detection | DONE |
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

Security events now record session creation/revocation and refresh-token reuse; refresh replay revokes the affected session and writes the high-severity event in the same database transaction.

Remaining before Sprint 03 can close: passkey/WebAuthn credentials and a shared authenticated HTTP boundary to remove repeated Bearer parsing.

### Profiles / social graph

Implemented: username/profile/bio/interests, public profile, edit profile, follow/unfollow, block/unblock, follower/following lists, shared-interest context, self-profile canonical routing and direct-message entry from profiles. People Discovery recommends non-followed/non-blocked users using mutual connections and shared interests.

Public profiles and their public posts are guest-readable. Follow/message/block remain authenticated actions and use a safe internal return path through login/onboarding.

Remaining: explicit mutual/friend semantics only if product validation still requires a distinct relationship; composite pagination only when measured scale requires it.

### Messaging / reliability

Implemented: unique direct chats, group reuse of the same messaging core, ordered per-chat sequence, `client_message_id` idempotency, cursor history, unread/read cursor, optimistic sending, reply/edit/delete, IndexedDB durable outbox with legacy migration, retry after reconnect, responsive desktop/mobile UI and deep-link chat selection.

Message reactions are complete in the client: one batch query loads reaction aggregates for up to 100 messages, quick reactions are available on bubbles, and toggling refreshes only the affected message reaction state. This avoids N+1 reaction requests.

Realtime backend is implemented with short-lived one-use tickets, origin validation, compression disabled, bounded send queues, read limits, exponential reconnect+jitter and PostgreSQL LISTEN/NOTIFY fanout. `RealtimeBridge` is active; REST polling remains the recovery path and is intentionally retained until realtime/load validation is measured on deployment hardware.

### Stranger requests / anti-spam

Direct-chat request state is part of the same chat model (`pending/accepted/rejected`). A stranger can send only one initial request message; the recipient must accept before conversation continues. Request Inbox shows contextual trust information and supports accept/reject without duplicating chats.

### Groups

Implemented: owner/admin/member, common messaging core, create/details, members, promote/demote admin, remove member, hashed expiring/limited invites, join, revoke invite, atomic owner transfer and leave-after-transfer.

Owner/admin group moderation is implemented server-side with object-level checks and in the chat UI. A moderator action can soft-delete a message only from the actor's group; ordinary members cannot perform it.

Per-action abuse thresholds remain part of Security Plane evolution rather than Group feature scope.

### Media / posts

Secure media pipeline has metadata/ownership/state controls, file validation, image decode/re-encode path, EXIF stripping intent, access checks and cleanup compensation for failed metadata persistence. Media does not become public merely by upload. Post-media attachment validates `ready` state and ownership transactionally.

Posts support thought/photo/post, visibility, replies, reactions, saves, soft deletion, permalink and profile rendering. Guest read uses a separate `PublicStore` SQL path that can only select `visibility='public'`; follower/private rules stay in authenticated queries. Public post media can be fetched anonymously only when backend media authorization confirms it is linked to public content.

### Feed / Discovery / creator growth

Feed modes: `for-you` and `following`. Ranking is explainable and excludes blocked relationships. Signals include follows, shared interests, discussion activity, explicit feedback and exploration bonus for fresh content from smaller authors. User feedback persists as `more_like_this`, `not_interested` or `hide`; hard negative feedback removes posts from future candidate selection.

Discover includes people recommendations. New creators receive bounded exploration bonus without fake followers/reactions or guaranteed popularity.

### Communities / channels

Communities reuse normal group chats for live conversation. Public communities enter catalog only after moderation approval. Membership and chat membership remain synchronized. Approved public community pages/catalog are guest-readable; Join/chat access remains authenticated. Private or pending communities never enter the guest query path.

Channels use the shared posts engine for broadcast content, maintain subscribers and moderation state, and support catalog/create/profile/subscription/owner publishing flows. Approved public channels and their public posts are guest-readable through a separate public query path; pending/rejected owner content remains authenticated. Subscribe/publish are authenticated actions.

Moderation decisions update the underlying public entity state rather than merely closing a case.

### Search

Global search covers people, public posts and only groups where the viewer is already a member. Private group metadata is intentionally excluded from global discovery. Communities/channels use their moderated catalogs rather than leaking private entities into search.

### Notifications / PWA

Durable notification inbox, unread counts, mark-read/all-read, push subscription storage, notification generation for messages/follows/replies/channel posts, PWA manifest and service worker are implemented. Service worker intentionally never caches authenticated `/api/*` responses. Web Push cryptographic delivery is not custom-built; it remains gated on a vetted library and VAPID configuration.

### Trust / moderation / admin

Report flow is `Report → Case → Decision → Enforcement`, not direct user-triggered bans. Admin identities/roles are separate from ordinary users. Moderation queue uses RBAC and metadata-first review; ordinary admin access does not expose arbitrary private conversations. Critical admin actions are written to DB-enforced append-only audit tables.

### Security plane

Append-only `security_events` storage is active. Producers currently cover session creation/revocation, refresh-token replay and aggregated rate-limit spikes. Rate-limit events are deduplicated to at most one event per bucket/IP/minute so an attacker cannot turn detection into a database write DoS. No message content is stored in these technical events.

High/critical security events are routed by a database trigger into an operational alert queue. Security/owner roles can acknowledge alerts with a mandatory note; acknowledgement is audited. Runtime operational controls can temporarily pause Feed, Discovery, new Posts, Community creation, Channel creation or Media upload without disabling Messaging. Each flag change writes both append-only admin audit and security events. Multi-node distributed rate limiting/correlation remains a future scale-out task.

### Offline / weak network

Versioned IndexedDB outbox is active with retry/idempotency. Data Saver modes are implemented: Auto, Save, Maximum. Auto reacts to browser `saveData`/2G signals. Maximum mode prevents authenticated/public media components from downloading images until the user explicitly requests them.

### Public web / growth

Public profile, public post permalink/replies/media, approved channels and approved public communities are readable without account creation. Interaction actions remain authenticated. Login stores only a validated internal `return_to` path in sessionStorage; after login/onboarding users return to the original post/profile/channel/community instead of losing acquisition context. The helper rejects external/open-redirect paths.

### Edge / security hardening

Implemented: strict CORS allowlist, secure headers, no-store for API, panic recovery, request IDs, request telemetry, bounded single-node rate limiter with Retry-After, no trust of arbitrary forwarded client IP headers, WebSocket-safe telemetry handling, non-root scratch backend Docker image and dependency-locked websocket library.

Production configuration is fail-closed: explicit database URL is required, development database credentials are rejected, web/magic-link/Yandex URLs require HTTPS, production SMTP is mandatory and example.com sender values are rejected.

Migration audit fixed duplicate migration versions and the `security_events.session_id` foreign key now references the actual `sessions` table.

Operational artefacts now include:
- `docs/SECURITY_INCIDENT_RUNBOOK.md`
- `docs/RELEASE_CHECKLIST.md`
- `ops/release/check.sh`
- `docs/DISASTER_RECOVERY.md`

### Performance / DR

A Go load harness supports health bursts, idempotent message bursts and WebSocket connection tests and reports RPS/p50/p95/p99. Actual server capacity numbers must be measured on deployment hardware before Sprint 25 closes.

DR scripts create integrity-checked PostgreSQL custom-format backups and media archives and require an explicit separate target + destructive confirmation for restore. Sprint 27 closes only after an actual restore drill succeeds.

## Remaining release-critical work

### Sprint 03

- Passkey/WebAuthn using a vetted maintained implementation; do not invent cryptography.
- Shared auth middleware/boundary.

### Sprint 05

- Decide whether a distinct `friend` relation is still necessary based on Alpha behavior.
- Composite pagination when measured usage requires it.

### Sprint 06–07

Functional realtime and messaging scope is complete: `chat:realtime` triggers immediate chat/list synchronization, Hub metrics track active connections/published/dropped events, and REST remains the recovery path. Polling reduction remains a measured Sprint 25 optimization after reconnect/load validation, not a correctness blocker.

### Sprint 21 / 26

Sprint 21 single-node Security Plane is functionally complete: append-only events, rate-limit/session producers, high/critical alert routing, acknowledgement workflow, admin audit, security summary and operational public-feature controls are implemented.

Remaining Sprint 26/deployment work:
- Production secrets manager integration in deployment environment.
- Verify production reverse proxy/network segmentation on the target host.
- Independent pentest remains a mandatory Public Beta gate.

### Sprint 24

- Public read/acquisition surfaces are implemented.
- Remaining work: OpenGraph/server-rendered share metadata, acquisition analytics and measured creator/community migration funnel.

### Sprint 25

- Run health/message/WebSocket profiles on actual server.
- Record concurrency ceiling, CPU/RAM, p95/p99, reconnect-storm behavior.
- Define degradation thresholds for Feed/Discovery before Messaging.

### Sprint 27

- Run backup + restore drill on isolated target.
- Record RPO/RTO and verify media/database consistency.

## Known deployment blockers before Closed Alpha

- Generate and commit a real `web/package-lock.json` using npm; never fabricate it manually.
- Execute `ops/release/check.sh` in an environment with Go/npm.
- Run clean-database migrations from zero.
- Run the load harness against actual deployment hardware.
- Execute a real database/media restore drill.
- Verify production SMTP/Yandex values without committing secrets.

## Closed Alpha gate (Sprint 28)

Closed Alpha may start only when all are true:

- Clean database migrations succeed from zero.
- Backend formatting/vet/race tests pass locally/server-side.
- Frontend lockfile/typecheck/build pass.
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

Complete the remaining Sprint 03 passkey dependency lock only through a reproducible Go module update, then finish Sprint 24 acquisition analytics/share funnel. In parallel, the deployment environment must generate the frontend npm lockfile and execute the release gate, clean migrations, load tests and restore drills before Sprint 28 can start.
