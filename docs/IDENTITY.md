# CHAT Identity and Session Model

## Goals

- Fast email-first account creation without leaking whether an account already exists.
- One-time, short-lived magic links.
- Server-revocable sessions with separate short-lived access and long-lived refresh credentials.
- Architecture ready for Yandex ID and WebAuthn/passkeys without changing the user/account model.

## Email magic-link flow

1. Client calls `POST /api/v1/auth/email/start` with an email.
2. Backend normalizes and validates the address, checks attempt limits, generates 32 cryptographically random bytes and stores only SHA-256(token).
3. Development transport logs the magic link. Production is deliberately fail-closed until a real approved mail transport is configured.
4. Client submits the token to `POST /api/v1/auth/email/complete`.
5. Backend atomically locks and consumes the challenge; replay/expired tokens fail.
6. Existing email identity resolves its user. Otherwise a new user + verified email identity is created.
7. Backend creates a server-side session with independent access/refresh token hashes.
8. Access token is returned to the client; refresh token is sent as HttpOnly SameSite cookie.

## Security rules

- Raw login/access/refresh tokens are never persisted.
- Login token TTL: 15 minutes.
- Access token TTL: 15 minutes.
- Refresh TTL: 30 days.
- Login attempts are limited per email/IP window.
- Public responses must not disclose whether an account exists.
- Forwarded IP headers are not trusted until traffic is normalized by a configured trusted edge proxy.
- Production may not use the log magic-link transport.

## Future identity providers

`user_identities` already supports provider bindings. Yandex ID will create/link `provider='yandex'` identities after OAuth state/PKCE validation. Phone identification can be added as a separate binding/activation requirement without making phone the primary account identifier.

## Passkeys

WebAuthn credentials will be bound to a user account after an authenticated/verified setup ceremony. Privileged admin identities will require passkeys as a separate security domain.
