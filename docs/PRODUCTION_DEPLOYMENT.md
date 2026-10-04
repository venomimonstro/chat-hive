# CHAT production deployment runbook

This runbook is intentionally compatible with the project's direct-to-main/no-GitHub-Actions delivery mode. It does not weaken release gates.

## 1. Host prerequisites

- Linux host with Docker Engine and Docker Compose v2.
- Public DNS A/AAAA for `CHAT_PUBLIC_HOST` pointing to the host.
- TCP 80/443 and UDP 443 reachable by Caddy.
- No public PostgreSQL, Redis or NATS ports.
- Enough persistent disk for PostgreSQL, media and backups.
- Server clock synchronized.

## 2. Secrets and configuration

1. Copy `.env.production.example` to `.env.production`.
2. Replace every placeholder secret with a unique high-entropy value.
3. Never store `.env.production` in Git.
4. Keep `NEXT_PUBLIC_API_BASE_URL` blank for the normal same-origin deployment.
5. Keep `CHAT_TRUSTED_PROXY_CIDRS=172.31.238.2/32` unless you intentionally change the fixed Caddy address in `docker-compose.prod.yml`; do not widen it to the whole edge subnet.
6. Configure SMTP. Production API fails closed if SMTP is missing.
7. Configure Yandex ID only after the OAuth application redirect URI exactly matches the public HTTPS callback.

## 3. Frontend dependency lock

A real `web/package-lock.json` is mandatory.

Run from `web/` in an environment with npm:

```sh
bash ../ops/frontend/lock.sh
```

Commit the generated lockfile. Do not hand-write it.

Then build the web image:

```sh
docker build -t chat-web:local ./web
```

## 4. Local/server release gate

From the repository root:

```sh
sh ops/release/check.sh
```

Do not deploy if the gate reports a blocking failure.

## 5. Start production stack

```sh
docker compose --env-file .env.production -f docker-compose.prod.yml pull
docker compose --env-file .env.production -f docker-compose.prod.yml build api migrate
docker compose --env-file .env.production -f docker-compose.prod.yml up -d
```

The `migrate` service must complete successfully before API starts.

## 6. Health verification

From the host:

```sh
curl -fsS https://$CHAT_PUBLIC_HOST/health/live
curl -fsS https://$CHAT_PUBLIC_HOST/health/ready
```

Expected status is HTTP 200.

Also verify:

- HTTPS certificate is valid.
- HTTP redirects to HTTPS.
- `/api/*` is served by API.
- ordinary web paths are served by Next.js.
- WebSocket can obtain a one-use ticket and connect over WSS.
- DB/Redis/NATS are not reachable from the public internet.

## 7. First owner bootstrap

First create and verify a normal CHAT account by email. Before granting `owner` or `security`, sign in to that account and add at least one passkey in Security Center. Privileged roles cannot be granted without an existing passkey.

Then run the audited CLI once:

```sh
docker compose --env-file .env.production -f docker-compose.prod.yml run --rm \
  -e CHAT_ADMIN_BOOTSTRAP_CONFIRM=I_UNDERSTAND \
  --entrypoint /chat-adminctl api \
  -email owner@example.ru -role owner -reason "initial production owner"
```

Remove `CHAT_ADMIN_BOOTSTRAP_CONFIRM` immediately after use.

The operation is stored in `admin_audit_events`.

After the role is granted, sensitive owner/security Admin endpoints require a passkey-authenticated session. Email/Yandex sessions remain valid for ordinary CHAT usage and recovery, but cannot perform privileged Admin operations. Adding or deleting passkeys on a privileged account also requires an existing passkey-authenticated session.

## 8. Backup before every risky change

Follow `docs/DISASTER_RECOVERY.md`.

A backup is not considered valid until its archive/checksum has been verified and a restore drill has succeeded on an isolated database.

## 9. Operational controls

Security/owner/system-admin roles can pause selected public/heavy features from the Security Console. Messaging is intentionally not controlled by these switches.

Every flag change:
- requires a reason;
- is written to append-only admin audit;
- emits a security event;
- high-severity changes enter the security alert queue.

## 10. Incident handling

Use `docs/SECURITY_INCIDENT_RUNBOOK.md`.

Never erase evidence or audit records during an incident. Rotate compromised credentials, constrain blast radius and restore from verified backups as documented.

## 11. Closed Alpha gate

Do not call the deployment Closed Alpha ready until all conditions in `docs/MASTER_PLAN.md` and `docs/RELEASE_CHECKLIST.md` are satisfied, including:
- clean migrations from zero;
- Go tests/vet/race;
- frontend lock/typecheck/build;
- measured load profile on the actual host;
- successful restore drill;
- no unresolved Critical/High exploitable security issue.
