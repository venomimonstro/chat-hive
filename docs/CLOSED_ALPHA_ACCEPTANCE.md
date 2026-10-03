# Closed Alpha acceptance

Sprint 28 is an operational gate, not a declaration based only on code being present.

## Automated smoke

After deployment:

```sh
CHAT_ALPHA_URL=https://chat.example.ru sh ops/alpha/check.sh
```

For additional authenticated checks, supply a short-lived access token from a dedicated Alpha test account:

```sh
CHAT_ALPHA_URL=https://chat.example.ru \
CHAT_ALPHA_ACCESS_TOKEN='short-lived-token' \
sh ops/alpha/check.sh
```

Never commit the token.

The smoke script validates:

- HTTPS-only target;
- live/readiness health;
- public web routing;
- HSTS/nosniff/frame headers;
- no-store API responses;
- protected endpoint authentication;
- refresh without cookie rejection;
- absence of arbitrary CORS origin reflection;
- optional authenticated session/chat reads.

## Required manual product scenarios

Use two normal Alpha accounts plus one group admin account.

1. Email magic-link login.
2. Yandex ID login when configured.
3. Onboarding, interests and public profile.
4. Follow/unfollow and block/unblock.
5. Stranger request: only one initial message before acceptance.
6. Accept/reject request.
7. Direct message online.
8. Send while offline, reload, reconnect, verify exactly-once delivery.
9. Reply/edit/delete/reaction.
10. Create group, invite second user, promote admin, transfer ownership.
11. Admin removes another member's group message; normal member cannot.
12. Publish Thought/Post/Photo.
13. Public permalink works logged out; follower-only content does not leak.
14. Discover feedback: more/less/hide.
15. Create public Community/Channel; verify pending content is absent from guest catalog until moderation approval.
16. Notification inbox/read states.
17. Data Saver Maximum does not download media before explicit action.
18. Revoke another session from Trust Center.
19. Reuse an already rotated refresh token in an isolated test and confirm session revocation/security alert.
20. Operational public-feature control pauses the selected public feature while direct messaging remains usable.

## Mandatory infrastructure evidence

Record in an Alpha release note:

- Git commit SHA.
- Output of `ops/release/check.sh`.
- clean migration result from an empty database;
- load-test host CPU/RAM and p50/p95/p99;
- WebSocket concurrent-connection ceiling;
- reconnect-storm behavior;
- backup timestamp/checksum;
- isolated restore result;
- measured RPO/RTO;
- open Critical/High findings (must be zero exploitable findings);
- SMTP and Yandex configuration verification;
- smoke script output.

## Exit criteria

Closed Alpha may be declared ready only when the gates in `docs/MASTER_PLAN.md` and `docs/RELEASE_CHECKLIST.md` are all satisfied.

Failure of a growth, Feed or Discovery feature may degrade that feature. Failure of those components must not prevent direct messaging from operating.
