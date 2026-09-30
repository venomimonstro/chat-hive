# CHAT — MASTER PLAN

This file is the single source of truth for product and engineering delivery.

## Product invariant

CHAT is a mobile-first social messenger. The primary experience is fast private communication; discovery, profiles, communities and creator growth extend communication rather than replace it.

Permanent product constraints:

1. Four primary navigation areas maximum: Chats, Discover, Create, Me.
2. Messaging remains usable if feed/discovery is degraded.
3. No political/religious profiling of users.
4. Public content and private communication use separate moderation policies.
5. Security, permissions, rate limits and abuse cases are part of every feature Definition of Done.
6. No production secrets or production user data are exposed to AI development agents.
7. Database changes require migrations.
8. Working modules are extended, not duplicated or silently replaced.

## Architecture decision

- Backend: Go modular monolith.
- Realtime: WebSocket gateway; durable state remains PostgreSQL-backed and REST sync remains a recovery path.
- Frontend: Next.js + TypeScript, mobile-first PWA.
- Source of truth: PostgreSQL.
- Ephemeral/cache/presence: Redis.
- Event bus/background processing: NATS JetStream.
- Media: S3-compatible object storage + CDN.
- Deployment: Docker first; no Kubernetes until operational load justifies it.

## Delivery mode

Development currently proceeds directly in `main` at owner request. GitHub Actions/CI workflow has been removed. Code-quality gates remain part of the Definition of Done and must be run locally/server-side before production deployment even while repository development is direct-to-main.

## Sprint status

| Sprint | Scope | Status |
|---|---|---|
| 00 | Product/architecture/security source of truth | DONE |
| 01 | Repository, local infrastructure, backend/frontend skeleton | DONE |
| 02 | Design system and responsive application shell | DONE |
| 03 | Identity: email magic-link, session rotation/revocation, Yandex ID, SMTP, passkeys | IN PROGRESS |
| 04 | Onboarding: username, profile, interests | DONE |
| 05 | Profiles and social graph | IN PROGRESS |
| 06 | Realtime gateway | PLANNED |
| 07 | Direct messaging core | IN PROGRESS |
| 08 | Reliable messaging/offline outbox/idempotency | IN PROGRESS |
| 09 | Image/media pipeline | PLANNED |
| 10 | Groups | IN PROGRESS |
| 11 | Posts: thought/photo/post | IN PROGRESS |
| 12 | Feed | IN PROGRESS |
| 13 | Discovery | IN PROGRESS |
| 14 | New-creator exploration distribution | PLANNED |
| 15 | Communities | PLANNED |
| 16 | Channels | PLANNED |
| 17 | Global search | IN PROGRESS |
| 18 | Stranger requests + anti-spam | PLANNED |
| 19 | Moderation core | PLANNED |
| 20 | Admin console | PLANNED |
| 21 | Security plane/detection | PLANNED |
| 22 | Trust center | IN PROGRESS |
| 23 | Offline + low-data modes | IN PROGRESS |
| 24 | Public web + invitation growth loops | PLANNED |
| 25 | Performance/load profiling | PLANNED |
| 26 | Security hardening + independent pentest gate | PLANNED |
| 27 | Backup/disaster recovery validation | PLANNED |
| 28 | Closed alpha | PLANNED |
| 29 | Product iteration from measured activation/retention | PLANNED |
| 30 | Public beta gate | PLANNED |

## Implemented foundation

The repository contains architecture/security/data/API rules, local PostgreSQL/Redis/NATS infrastructure, Go API, Next.js mobile-first shell and reusable design system.

Identity currently includes persistent email magic links, hashed one-time challenges, short-lived access credentials, HttpOnly refresh credentials, server-side validation/revocation, refresh rotation with replay detection, Yandex ID Authorization Code + PKCE flow, production SMTP/STARTTLS transport and active-device/session listing. Yandex access tokens are not persisted in the browser or CHAT database.

Onboarding includes username, display name, bio, persistent interests and atomic completion. Social Graph includes public profile lookup, follow/unfollow, block/unblock, self-profile routing, profile editing and real profile publications.

Messaging uses PostgreSQL as durable source of truth. Direct chat creation is unique per user pair, blocked users cannot exchange messages, each chat has an ordered sequence, client_message_id provides retry idempotency, message history is sequence-cursor based and read position is server-side. The Next.js messenger has responsive desktop/mobile layout, deep-link chat selection, optimistic sending, persistent offline outbox, reply/edit/delete UI and older-history pagination. Backend reactions are implemented; chat reaction UI remains.

Groups reuse the same chat/message model. Owner/admin/member roles, hashed limited/expiring invite tokens, member management, group creation/details and invite/join web flows are implemented. Remaining group work is owner transfer, moderation controls and rate limits.

Posts now have durable schema and API for thoughts/posts, visibility, replies, reactions, saves and soft deletion. Web includes Create Hub, thought/post composer, permalink, discussion thread and profile publication list. Photo publishing intentionally waits for Sprint 09 media pipeline.

Feed/Discover has an explainable rule-based backend and mobile-first screen. Modes `for-you` and `following` are implemented. Current ranking uses follows, shared interests and discussion activity while excluding blocked relationships. User-facing recommendation reasons are returned by the backend.

Important schema correction: migration 000007 is a self-contained messaging + groups migration and creates chats, chat_members, direct_chat_pairs, messages, reactions and group_invites in dependency-safe order. Migration 000008 creates publishing tables.

## Sprint 03 remaining

- Passkey/WebAuthn credential storage and endpoint foundation.
- Consolidate repeated Bearer authentication parsing behind one shared authenticated HTTP boundary.
- Add security/audit events for login/session-sensitive operations.

## Sprint 05 remaining

- Followers/following list endpoints and screens.
- Mutual/friend relationship semantics after product review.

## Sprint 06 remaining

- Add WebSocket transport only with correctly locked dependency checksums.
- Authentication, heartbeat, reconnect, bounded connection limits and origin validation.
- Message events accelerate delivery but never replace PostgreSQL/REST recovery sync.

## Sprint 07–08 remaining

- Chat reaction UI without per-message N+1 requests.
- Upgrade browser outbox from localStorage to versioned IndexedDB with bounded retention.
- Composite pagination for chat list to avoid timestamp collision edge cases.
- Direct chat lookup endpoint for deep links outside first chat-list page.

## Sprint 10 remaining

- Owner-transfer flow before owner can leave.
- Group-level moderation controls and rate limits.

## Sprint 11 remaining

- Photo posts after secure media pipeline.
- Improve post edit response to include canonical author fields.
- Composite post cursor `(created_at,id)` to remove timestamp-collision edge cases.
- Reaction state/toggle UI without optimistic double-count drift.

## Sprint 12–13 remaining

- People discovery/search.
- Community discovery after Sprint 15 exists.
- Hide/not-interested feedback and recommendation preference signals.
- Add feed pagination UI and composite cursor.

## Sprint 17 current gap

The Discover search button points to `/search`. Implement the route and backend before considering this sprint complete; no dead navigation is acceptable.

## Definition of Done for every feature sprint

A feature is not DONE until all applicable items are satisfied:

- Product flow and error/empty/loading states are implemented.
- Mobile and desktop behavior are defined.
- Authentication and object-level authorization are enforced server-side.
- Input validation and output encoding are present.
- Rate limits/abuse scenarios are considered.
- State-changing operations are idempotent where retries are possible.
- Audit/security events are emitted where required.
- Unit/integration/regression tests cover the critical path.
- Migrations exist for schema changes and are dependency-safe from a clean database.
- Documentation and this sprint table are updated.
- No known Critical/High exploitable security issue is introduced.

## AI development protocol

Before implementing a sprint, an AI agent must:

1. Read this file and relevant architecture/security/data docs.
2. Audit current implementation before creating new modules.
3. Preserve working behavior unless the sprint explicitly replaces it.
4. Minimize new dependencies and justify each one.
5. Never invent custom cryptography.
6. Never bypass validation/security controls to make code appear complete.
7. Never use production credentials/data.
8. Update documentation and sprint status after implementation.

## Current next action

Close live search and follower/following screens, then finish group owner transfer and IndexedDB offline queue. After durable messaging/social flows are clean, proceed to secure media pipeline; realtime transport follows once dependency locking can be generated and verified correctly.
