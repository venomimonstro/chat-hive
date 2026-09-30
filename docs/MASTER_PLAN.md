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
| 11 | Posts: thought/photo/post | PLANNED |
| 12 | Feed | PLANNED |
| 13 | Discovery | PLANNED |
| 14 | New-creator exploration distribution | PLANNED |
| 15 | Communities | PLANNED |
| 16 | Channels | PLANNED |
| 17 | Global search | PLANNED |
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

Onboarding includes username, display name, bio, persistent interests and atomic completion. Social Graph includes public profile lookup, follow/unfollow and block/unblock with server-side authorization semantics.

Messaging now uses PostgreSQL as durable source of truth. Direct chat creation is unique per user pair, blocked users cannot exchange messages, each chat has an ordered sequence, client_message_id provides retry idempotency, message history is cursor-based and read position is server-side. The Next.js messenger has responsive desktop/mobile layout, optimistic sending and a browser-persisted outbox that reuses client_message_id after reconnect/reload.

Groups reuse the same chat/message model. The schema supports owner/admin/member roles and hashed limited/expiring invite tokens. Group service/store/API foundation implements create, membership listing, role changes, removal, leave, invite/join/revoke flows with transactional permission checks.

Important schema correction: migration 000007 is a self-contained messaging + groups migration and creates chats, chat_members, direct_chat_pairs, messages, reactions and group_invites in dependency-safe order.

## Sprint 03 remaining

- Passkey/WebAuthn credential storage and endpoint foundation.
- Consolidate repeated Bearer authentication parsing behind one shared authenticated HTTP boundary.
- Add security/audit events for login/session-sensitive operations.

## Sprint 05 remaining

- Followers/following list endpoints and screens.
- Profile edit screen after onboarding.
- Mutual/friend relationship semantics after product review.

## Sprint 07–08 remaining

- Message edit/delete/reply/reactions API and UI.
- Older-history pagination UI.
- Replace temporary polling with realtime transport while retaining REST recovery sync.
- Move production-grade offline queue from simple localStorage foundation to IndexedDB with bounded retention and migration/versioning.

## Sprint 10 remaining

- Group UI: creation, details, member list, role management and invite sharing.
- Owner-transfer flow before owner can leave.
- Group-level moderation controls and rate limits.

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

Finish Sprint 10 group UI and permission edge cases, then complete direct-message actions and history pagination. After the durable messaging surface is complete, add realtime transport as an acceleration layer rather than as the source of truth.
