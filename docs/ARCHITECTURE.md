# CHAT Architecture

## Goals

The system must start cheaply, remain operational on constrained infrastructure, and scale horizontally without rewriting the product core.

## Runtime topology

```text
Internet
  -> anti-DDoS/WAF/reverse proxy
    -> Web/API
    -> WebSocket gateway
       -> CHAT Core modules
          -> PostgreSQL (source of truth)
          -> Redis (cache/presence/rate limits only)
          -> NATS JetStream (async events/jobs)
             -> notification/media/moderation workers
          -> object storage/CDN for media
```

## Module boundaries

Initial Go modules/packages:

- identity: accounts, identities, sessions, login flows.
- users: account state and lifecycle.
- profiles: username, avatar, bio, interests/privacy presentation.
- socialgraph: follows, blocks, later mutual/friend state.
- chats: chat membership and chat metadata.
- messaging: durable messages, reads, replies, reactions.
- groups: group lifecycle and roles.
- posts: public user content.
- feed: ordered feed retrieval.
- discovery: candidate generation/scoring.
- communities/channels: public/community surfaces.
- media: upload metadata and processing state.
- moderation/risk: reports, cases, actions, signals.
- notifications: durable notification intent and delivery jobs.
- audit: immutable security/privileged activity events.

Modules may share one deployable initially but must not bypass module ownership by directly mutating each other's tables from arbitrary handlers.

## Data ownership

PostgreSQL is authoritative. Redis loss must never lose acknowledged user messages or account state. NATS consumers are at-least-once tolerant and must be idempotent.

## Reliability rules

- State-changing client operations use stable client operation/message IDs where retries are possible.
- Cursor pagination is mandatory for unbounded lists/history; no large OFFSET pagination.
- Acknowledged messages are persisted before success acknowledgement.
- Reconnect uses exponential backoff with jitter to prevent reconnect storms.
- Feed/discovery/analytics can be degraded before messaging/authentication.

## Scaling path

Phase 1: API + gateway + workers on one host with PostgreSQL/Redis/NATS.

Phase 2: split public edge, API, gateway and workers; media remains external object storage.

Phase 3: N API/gateway/worker nodes behind load balancers, PostgreSQL replicas/partition strategy, Redis/NATS clustering when load proves necessary.

No Kubernetes is introduced only for appearance; deployment complexity must be justified by measured operational need.

## Frontend

Next.js App Router + TypeScript. The web client is mobile-first and progressively enhanced as PWA. Recent data and outbox will later live in IndexedDB. The UI is constrained to four permanent destinations: Chats, Discover, Create, Me.
