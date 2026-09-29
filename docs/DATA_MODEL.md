# CHAT Initial Data Model

This document defines ownership boundaries, not the final schema.

## Identity and users

- `users`: account lifecycle/status and timestamps.
- `user_identities`: email/Yandex/phone-ready identity bindings.
- `sessions`: server-side session registry and revocation state.
- `profiles`: username, display name, bio, avatar reference and privacy settings.
- `interests` / `user_interests`: explicit onboarding interests only.

## Social graph

- `follows`: one-way public social relationship.
- `blocks`: safety/privacy relationship that overrides discovery/messaging.

## Messaging

- `chats`: chat metadata/type.
- `chat_members`: membership/role/read cursor.
- `messages`: durable ordered chat messages.
- `message_reactions`: user reaction to a message.
- `attachments`: references to media objects; binary data never lives in PostgreSQL.

Messages use cursor/sequence based history. `client_message_id` is unique per sender/session scope to make retries idempotent.

## Public content

- `posts`: canonical thought/photo/post object.
- `post_media` / `post_reactions` / `post_comments`.
- `communities` / `community_members`.
- `channels` / `channel_subscribers`.

## Safety and administration

- `reports`.
- `moderation_cases`.
- `moderation_actions`.
- `risk_events`.
- `admin_users` / `admin_roles`.
- `audit_events` (prefer append-only/separate storage path as system matures).

## Data rules

1. PostgreSQL is authoritative for acknowledged account/message state.
2. Binary media lives in object storage and is referenced by immutable media IDs.
3. Redis is never the sole copy of durable user content.
4. Sensitive auth tokens are stored as hashes where verification permits.
5. Political/religious inferred profiles are prohibited by product architecture.
6. Schema changes are migration-only and must be backwards-aware for rolling deployments.
