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
- Realtime: WebSocket gateway, initially deployable with the API but separated by package/process boundaries.
- Frontend: Next.js + TypeScript, mobile-first PWA.
- Source of truth: PostgreSQL.
- Ephemeral/cache/presence: Redis.
- Event bus/background processing: NATS JetStream.
- Media: S3-compatible object storage + CDN.
- Deployment: Docker first; no Kubernetes until operational load justifies it.

## Sprint status

| Sprint | Scope | Status |
|---|---|---|
| 00 | Product/architecture/security source of truth | DONE |
| 01 | Repository, local infrastructure, backend/frontend skeleton, CI | DONE |
| 02 | Design system and responsive application shell | IN PROGRESS |
| 03 | Identity: email magic-link, sessions, Yandex ID foundation, passkeys | PLANNED |
| 04 | Onboarding: username, avatar, interests, initial discovery seed | PLANNED |
| 05 | Profiles and social graph | PLANNED |
| 06 | Realtime gateway | PLANNED |
| 07 | Direct messaging core | PLANNED |
| 08 | Reliable messaging/offline outbox/idempotency | PLANNED |
| 09 | Image/media pipeline | PLANNED |
| 10 | Groups | PLANNED |
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
| 22 | Trust center | PLANNED |
| 23 | Offline + low-data modes | PLANNED |
| 24 | Public web + invitation growth loops | PLANNED |
| 25 | Performance/load profiling | PLANNED |
| 26 | Security hardening + independent pentest gate | PLANNED |
| 27 | Backup/disaster recovery validation | PLANNED |
| 28 | Closed alpha | PLANNED |
| 29 | Product iteration from measured activation/retention | PLANNED |
| 30 | Public beta gate | PLANNED |

## Completed foundation

Sprint 00–01 was merged only after CI passed frontend typecheck/build and backend formatting/vet/race tests. The repository now contains the authoritative architecture/security/data/API rules, local PostgreSQL/Redis/NATS stack, initial account/session migration, Go API skeleton and mobile-first Next.js shell.

## Sprint 02 acceptance criteria

- Design tokens cover color, spacing, typography, radius, motion and elevation.
- Reusable primitives exist for Button, IconButton, Avatar, Badge, SearchField, Surface/Card, Skeleton, EmptyState and Sheet/Modal foundation.
- Responsive `AppShell` owns the permanent four-zone navigation rather than individual pages.
- Light/dark modes are token-driven and accessible.
- Touch targets are at least 44px for primary interactive controls.
- Focus-visible states exist for keyboard accessibility.
- The existing Chats preview is refactored to use design-system primitives instead of one-off markup/styles.
- Frontend typecheck/build remain green.

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
- Migrations exist for schema changes.
- Documentation and this sprint table are updated.
- No known Critical/High exploitable security issue is introduced.

## AI development protocol

Before implementing a sprint, an AI agent must:

1. Read this file and relevant architecture/security/data docs.
2. Audit current implementation before creating new modules.
3. Preserve working behavior unless the sprint explicitly replaces it.
4. Minimize new dependencies and justify each one.
5. Never invent custom cryptography.
6. Never bypass failing tests/security controls to make CI green.
7. Never use production credentials/data.
8. Update documentation and sprint status after implementation.

## Current next action

Complete Sprint 02 Design System and responsive application shell, verify CI, then begin Sprint 03 Identity.