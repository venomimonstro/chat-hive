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
| 03 | Identity: email magic-link, session rotation/revocation, Yandex ID, SMTP, passkeys | DONE |
| 04 | Onboarding: username, profile, interests | DONE |
| 05 | Profiles and social graph | DONE |
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
| 24 | Public web + invitation growth loops | DONE |
| 25 | Performance/load profiling | IN PROGRESS |
| 26 | Security hardening + independent pentest gate | IN PROGRESS |
| 27 | Backup/disaster recovery validation | IN PROGRESS |
| 28 | Closed alpha | IN PROGRESS |
| 29 | Product iteration from measured activation/retention | IN PROGRESS |
| 30 | Public beta gate | IN PROGRESS |

## Implemented product surface

### Identity / account security

Implemented: persistent email magic links, hashed one-time challenges, short-lived access credentials, HttpOnly refresh credentials, server-side sessions, refresh rotation/replay detection, session revoke/listing, production SMTP/STARTTLS, Yandex ID Authorization Code + PKCE. Yandex OAuth tokens are not persisted in the browser or CHAT database.

Security events now record session creation/revocation and refresh-token reuse; refresh replay revokes the affected session and writes the high-severity event in the same database transaction.

Sprint 03 is functionally complete. Passkeys use `go-webauthn/webauthn v0.12.1` with checked module sums, server-side one-time ceremonies, discoverable login, credential lifecycle management, clone-warning security alerts and ordinary CHAT sessions after assertion. Authenticated HTTP modules use the shared `authhttp.Guard`; only the Identity package keeps its internal bearer parser because it implements the auth boundary itself. Real-device WebAuthn remains an Alpha smoke requirement, not an implementation gap.

### Profiles / social graph

Implemented: username/profile/bio/interests, public profile, edit profile, follow/unfollow, block/unblock, follower/following lists, shared-interest context, self-profile canonical routing and direct-message entry from profiles. People Discovery recommends non-followed/non-blocked users using mutual connections and shared interests.

Public profiles and their public posts are guest-readable. Follow/message/block remain authenticated actions and use a safe internal return path through login/onboarding.

MVP social graph is complete. CHAT intentionally uses Follow plus direct/group communication instead of inventing a separate Friend entity before Alpha evidence requires it. A distinct mutual/friend relation and composite pagination are deferred product/scale decisions, not release blockers.

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

Implemented: strict CORS allowlist, secure headers, no-store for API, panic recovery, request IDs, request telemetry, bounded single-node rate limiter with Retry-After, WebSocket-safe telemetry handling, non-root scratch backend Docker image and dependency-locked websocket library. Session records now carry an explicit authentication method; sensitive Admin operations for owner/security require a passkey-authenticated session. Privileged roles cannot be bootstrapped without an existing passkey, privileged passkey changes require passkey step-up, and passkey clone warnings fail closed. Behind production Caddy, client IP recovery is trusted-proxy aware: forwarded addresses are accepted only from the fixed Caddy peer `172.31.238.2/32`; direct users and the web container cannot spoof XFF for rate limits, auth abuse detection or admin audit.

Production configuration is fail-closed: explicit database URL is required, development database credentials are rejected, web/magic-link/Yandex URLs require HTTPS, production SMTP is mandatory and example.com sender values are rejected.

Migration audit fixed duplicate migration versions and the `security_events.session_id` foreign key now references the actual `sessions` table.

Operational artefacts now include:
- `docs/SECURITY_INCIDENT_RUNBOOK.md`
- `docs/RELEASE_CHECKLIST.md`
- `docs/PRODUCTION_DEPLOYMENT.md`
- `ops/release/check.sh`
- `docs/DISASTER_RECOVERY.md`

Production uses a single Caddy HTTPS ingress. Browser API requests default to same-origin; API/Web containers are not directly published, while PostgreSQL/Redis/NATS remain on an internal network. An audited `chat-adminctl` CLI grants the initial admin role only to an existing verified CHAT email account.

### Performance / DR

A Go load harness supports health bursts, real message-write bursts, WebSocket connection tests and reconnect-storm tests and reports RPS/p50/p95/p99. The harness exits non-zero on transport, WebSocket or unexpected HTTP failures. Protected runtime metrics expose active WebSocket connections, dropped realtime events and PostgreSQL pool pressure. Automatic hysteresis-based load shedding limits Feed/Discovery/Search/media at 85% DB pool utilization and recovers below 70%, while Messaging remains outside the shed class. Actual server capacity numbers must still be measured on deployment hardware before Sprint 25 closes.

DR scripts create integrity-checked PostgreSQL custom-format backups and media archives and require an explicit separate target + destructive confirmation for restore. `ops/backup/drill.sh` now runs a non-public isolated PostgreSQL restore, verifies critical durable tables and reports measured RTO. Sprint 27 closes only after this drill succeeds against an actual server backup.

## Remaining release-critical work

### Sprint 03

DONE. Email magic-link, rotating/revocable sessions, Yandex ID, SMTP and discoverable Passkeys are implemented. Passkey registration is available only to an authenticated user; login does not require exposing email/username first. Security Center supports add/list/delete, while email remains an independent recovery path.
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

DONE for MVP scope. Public profile/post/channel/community pages have server-rendered share metadata; invite links have privacy-safe OpenGraph metadata; login preserves validated internal return paths; and first-party acquisition events measure public view → login start → completed onboarding without external trackers or storing invite tokens. Growth events have a dedicated rate budget. Creator/community migration experiments belong to Sprint 29 measured product iteration rather than blocking Alpha.

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

## Release hardening work log — 2026-10-17

The following implementation work has landed directly on `main` without GitHub Actions or CI. These code changes **do not close** any release-gated sprint:

| Sprint | Implemented in code | Evidence still required |
| --- | --- | --- |
| 25 | Browser reconciles messages through REST on a reduced interval when WebSocket is healthy, falling back to faster polling when disconnected; background-tab polling is limited (`7d415dc`). | Server load results (p95/p99, CPU/RAM, concurrency ceiling, reconnect storm), measured regression and capacity decision. |
| 26 | Stale administrator/group role rendering after chat switching prevented (`6060456`); prior PWA private-page cache hardening is in place. | Production secrets, network review and an independent pentest plus remediation. |
| 27 | Backup/restore tooling exists. | Successful restore on an isolated host using a real production-like backup, with evidence for both database and media, RPO/RTO. |
| 28 | Fixed an undeclared TypeScript field in the WebSocket client and late-response contamination of other chats (`eb17fdf`); preserved sessions across transient auth outages (`036ed88`); introduced dependency-free service-worker privacy tests in the manual release gate (`b21b914`). | Generate and commit genuine `web/package-lock.json` using npm; clean migrations; frontend typecheck/build, Go race tests, real-device Passkey tests, and full Alpha scenario sign-off. |
| 29 | First-party dashboard and funnel instrumentation are already implemented. | Observe genuine Alpha cohort, document prioritized changes and re-measure activation/retention. |
| 30 | Technical gate script and operational runbooks already exist. | Passed Alpha, independent pentest, monitoring/incident verification and legal/compliance approval. |

Operational test commands (run from the repository root):

```bash
node --test web/tests/sw.test.cjs
bash ops/release/check.sh
bash ops/alpha/gate.sh
bash ops/beta/gate.sh
```

The Alpha/Beta gates require their documented environment values, live target hosts and external evidence. They must not be marked PASS from source inspection alone. Do not fabricate the npm lockfile or replace deterministic `npm ci` with an uncontrolled install in production.

## Security dependency update — October 17, 2026

The original pinned Next.js `15.2.4` is older than the security releases
fixing React Server Components vulnerabilities (2025) and the October 2026
advisories. The frontend dependency has been pinned to `15.5.28`, the
official October 14, 2026 Next.js 15 maintenance security release.
Source: https://nextjs.org/blog/nextjs-security-update-october-14-2026

The manual release gate rejects older Next.js branches and runs a production
dependency audit. **A genuine lockfile and a passing frontend build on a
network-enabled machine are still required before deployment.**

## October 17 implementation continuation

- Sprint 25: corrected the Go load harness to measure elapsed wall time only
  after responses complete, reject unexpected status codes/incomplete results,
  and validate WebSocket readiness; unit tests added (`1265e25`).
  Real server performance evidence is still required.
- Sprint 26: Web Locks now serializes rotating refresh cookies across tabs
  (`e03fda4`), with concurrency and outage tests (`b32cf08`).
  Production security review and independent pentest remain outstanding.
- Sprint 27: added archive traversal/symlink verification with offline tests
  (`b56d517`), isolated media extraction in the restore drill (`94313c6`),
  and safer preflight before destructive restore (`0c7ad0f`).
  Production-like restoration and measured RPO/RTO remain outstanding.
- Sprint 28: added `ops/release/prepare-lockfile.sh` to generate and
  validate real npm dependency locks on a machine with registry access.
  The lockfile must still be committed and a full release gate passed.

## October 17 continuation — source changes vs acceptance

**Code committed directly to main (no CI/Actions)**

- **Sprint 27 / backup integrity:** `910e1e3` fixes SHA-256 files referring to
  renamed staging directories and makes timestamp collisions fail rather than
  replacing data. Added an offline smoke test to the manual release gate.
  `a689cd1` blocks accidental restoration into the configured live
  database and destructive media restoration through symlinked ancestors.
- **Sprint 26 / auth resilience:** `08440cc` preserves refresh credentials on
  transient datastore errors; `2c403b0` returns a retryable 503 for
  authentication infrastructure outages without treating them as user logout.
  Tests cover invalid tokens separately from temporary backend failures.
- **Sprint 26 / subscription safety:** `c7f0b45` rejects push subscription
  URLs aimed at localhost, IP literals, internal hostnames and unsafe ports;
  delivery workers must still validate resolved destinations at send time.
- **Sprint 26–28 / dependency security:** `385aea3` upgrades Next.js from
  unpatched 15.2.4 to maintenance LTS 15.5.28; `1821285` aligns React and
  react-dom to 19.3.1 and constrains sharp to ^0.35.5. The manual release
  gate now checks minimum patched versions and audits production packages.
  Relevant advisory: https://nextjs.org/blog/nextjs-security-update-october-14-2026

**Evidence not yet available:** successful production Next.js build, genuine
npm lockfile checked into source control, Go race tests, load profiles against
the live Alpha host, isolated restore using a real backup, independent pentest,
real-device Passkeys, measured Alpha cohort and legal/operational sign-off.
No Sprint 25–30 has been reclassified DONE by source-only changes.

## Sprint 25–28 continuation: current source changes

The following corrections are committed directly to `main`:

- **Sprint 26 — media privacy:** `d84d4ef` disables browser caching for media
  responses until the server can unambiguously classify media as public.
  Prevents private group/message images surviving an account change in a
  shared browser cache.
- **Sprint 28 — offline messaging:** `a8a5a0e` updates the current-chat guard
  synchronously and prevents a delayed IndexedDB enqueue from displaying its
  message in a different conversation after navigation.
- **Sprint 26 — request tracing:** `4e358c5` accepts only bounded, safe
  client request IDs and generates random server-side IDs otherwise.
- **Sprint 25 — load evidence:** `74161d2` captures host CPU/memory/disk
  snapshots before/after the technical Alpha gate, enforces private report
  permissions, and derives the WebSocket hold duration from the connection
  count and issue rate rather than hardcoding a fragile limit.

No real production host, full dependency build or live disaster-recovery
scenario was tested by these commits. Sprints 25–30 remain **IN PROGRESS**.
The release remains blocked until a genuine npm lockfile is committed, the
Go/Next.js checks and clean migrations pass, real WebSocket capacity and DR
evidence is collected, and the remaining external gates are signed off.

## October 17 continuation — measurable gates and backup schedule

All changes below are source-level, committed directly to `main`:

- **Sprint 28**: `443f169` removes duplicate pages after concurrent
  pagination and prevents late reaction fetches from a previous chat from
  modifying the current conversation.
- **Sprint 27**: `8b608e6` adds an explicitly installed systemd daily backup
  timer and checks the promoted backup before reporting success. Installation
  on the owner's actual server, off-site copies, alerting and restore evidence
  remain mandatory.
- **Sprint 25**: `d648ff8` adds optional maximum p95 duration criteria for
  health, message writes, websocket connects, and reconnect tests. Set
  `CHAT_ALPHA_HEALTH_MAX_P95`, `CHAT_ALPHA_MESSAGE_MAX_P95`,
  `CHAT_ALPHA_WS_MAX_P95` and `CHAT_ALPHA_RECONNECT_MAX_P95`
  to agreed Go duration values such as `800ms` or `3s`. Their default
  `0s` means record latency without an arbitrary unapproved SLO;
  real measurements must establish acceptable values.

**Remaining open sprints: 25, 26, 27, 28, 29, 30 (six).**
No completion flag changes until corresponding host/device/cohort/external
acceptance evidence is recorded. Frontend lockfile and full build, production
load and restore drill, independent pentest and manual Beta sign-off remain
release blockers.

## October 17 incremental safety gates

- **Sprint 27:** `898f521` refuses incomplete backups when the media root is
  missing, with an explicit opt-in solely for genuinely media-free instances;
  `54756b0` performs this check before the expensive database dump.
  Offline regression coverage was added. A real isolated restore is still
  required to close the sprint.
- **Sprint 28:** `44d72e8` makes the clean-migration verification
  non-bypassable in the manual release gate. The actual full Go/Next.js build,
  device scenarios and Alpha acceptance record are still required.

**Remaining open sprints: 25, 26, 27, 28, 29, 30.**
`main` changes alone do not establish release readiness.

## October 17 — next hardening increments

- **Sprint 26:** `52e7301` maps missing media to 404 and refuses truncated
  files; `9087f97` additionally checks each served file's SHA-256 digest
  against PostgreSQL, rewinds on success, and tests equal-size corruption.
- **Sprint 27:** `4671ca3` rejects a media source accessed through symlinked
  paths and disallows nested backup/media directories, preventing recursive
  self-archiving and disk exhaustion. Offline smoke cases cover both.
- **Sprint 25:** `3f21cf8` detects half-open WebSocket connections after
  missed application-level pong events, reconnects, and includes browser
  regression tests in the manual release gate.

The above commits are source changes; the full Go race test, Next.js build,
real-host load measurement, actual media+database restore and security
assessment are **not yet verified**. Sprints 25–30 remain IN PROGRESS.

## October 17 — authentication, push, and WebSocket stability

Implemented in source and pushed directly to `main`:

- **Sprint 28:** `331ac1f` stops treating profile API errors as
  an absent onboarding profile; a failure now blocks submission with a
  retry button. Browser regression cases distinguish HTTP 503 and an
  incomplete-but-valid profile.
- **Sprint 26:** `7248f34` prevents an in-flight refresh from
  restoring a logged-out or replaced account's access token, including a
  stale 401 response. `7109acf` only removes the browser push
  subscription after the server confirms endpoint revocation; tests
  cover logged-out users, 503 failures, and successful revocation.
- **Sprint 27:** `1e9ab37` refuses installation of a root-run backup
  timer from a non-root-owned writable code checkout or credentials file.
- **Sprint 25:** `bdc9feb` adds a 15-second WebSocket handshake timeout
  and reconnection, with a regression case; `ecd619c` prevents stale
  sockets from canceling the newer connection's watchdog.

**Acceptance is still pending:** genuine `web/package-lock.json`,
successful frontend build and Go race tests, clean migrations, real-host
load and disaster-recovery runs, independent pentest, physical-device
Passkey testing, Alpha cohort observations and Beta sign-off.
**Open sprints: 25, 26, 27, 28, 29, 30.**
Do not mark them DONE based solely on source modifications.

## October 17 — measured correctness and recovery improvements

Implemented and committed directly to `main`, without CI:

- **Sprint 28:** `ced705c` fixes the browser `Headers` test harness and
  adds 401-versus-503 session tests. `5a82bc5` and `330db23` add bounded
  exponential retry delays (up to 5 minutes) for the durable offline outbox;
  manual sends bypass the delay and the existing IndexedDB queue is not
  discarded. Three retry algorithm regressions passed in an offline Node.js
  sandbox; this does not replace the full frontend build.
- **Sprint 25:** `8d91d2a` starts reporting WebSocket health only after
  server `ready`, with a separate readiness timeout and browser regression
  cases. `d2fa40f` calculates p95/p99 by nearest rank instead of a
  downward-biased floor and adds short-sample Go tests.
- **Sprint 27:** `cf9d3b1` stages media extraction before modifying the
  restore target, uses transactional `pg_restore --exit-on-error`, and
  adds offline negative/success restoration scenarios. A real restore drill
  with a PostgreSQL snapshot and media volume remains mandatory.

**Six open sprint acceptance gates remain (25–30).**
Actual Go race tests, Next.js production build, media/database restore on
isolated real infrastructure, the load profile, pentest, Alpha cohort and
legal/incident sign-off have not been verified. The frontend lockfile is
still required.

## Current next action

Sprint 03 and Sprint 24 are complete. Next focus is server-verified Sprint 25/27 evidence and Sprint 28 Closed Alpha readiness: generate and commit the frontend npm lockfile, run the release gate, clean migrations, real-device Passkey smoke, load tests and restore drill on the target host.


## Closed Alpha execution

Sprint 28 is now IN PROGRESS. The repository contains `ops/alpha/check.sh` for deployed HTTPS/health/security-header/auth-boundary smoke checks and `docs/CLOSED_ALPHA_RUNBOOK.md` for manual product validation. Alpha cannot move to DONE until the target server provides recorded evidence from release checks, clean migrations, WebAuthn device smoke, load/reconnect tests and an isolated restore drill.


## Sprint 29 measurement layer

Sprint 29 is IN PROGRESS before cohort data exists so measurement is ready on day one of Alpha. Owner-only Product Metrics are implemented in the Admin plane and combine durable database outcomes with privacy-minimized first-party acquisition events. The dashboard reports registration/onboarding/first-message/first-post/follow activity, active users, D1/D7 cohort retention, acquisition funnel and moderation pressure across 7/30/90-day windows. Sprint 29 closes only after real Alpha data drives documented product changes and those changes are re-measured.

## Sprint 30 Public Beta gate

Sprint 30 is IN PROGRESS. `ops/beta/gate.sh` executes the full Alpha technical gate and additionally requires passkey coverage for active owner/security accounts, zero open high/critical security alerts, zero critical moderation cases, an independent pentest report and legal/compliance sign-off evidence. Privileged Admin authorization itself also enforces passkey session step-up; credential presence alone is not treated as MFA enforcement. `docs/PUBLIC_BETA_GATE.md` defines the external evidence contract. Sprint 30 cannot be DONE from repository code alone.
