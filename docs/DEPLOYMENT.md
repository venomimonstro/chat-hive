# CHAT — Production / Closed Alpha deployment

This runbook is operational documentation. It does not replace the legal/security release gates in `RELEASE_CHECKLIST.md`.

## 1. Host prerequisites

- Linux host with current security updates.
- Docker Engine + Docker Compose plugin.
- Reverse proxy / TLS termination managed outside this compose file.
- DNS for the web and API origins.
- SMTP credentials and, when enabled, Yandex OAuth credentials.
- Backup destination that is not the same failure domain as the application host.

PostgreSQL, Redis and NATS must never be published to the public network. `docker-compose.prod.yml` keeps them on the internal `data` network.

## 2. Prepare secrets

```bash
cp .env.production.example .env.production
chmod 600 .env.production
$EDITOR .env.production
```

Replace every `CHANGE_ME` and example domain. The PostgreSQL password in `CHAT_DATABASE_URL` must match `POSTGRES_PASSWORD`.

Production config is fail-closed: the API rejects development DB credentials, non-HTTPS public origins and placeholder SMTP sender addresses.

Do not commit `.env.production`.

## 3. Generate and commit the frontend lockfile

The repository intentionally does not accept floating production dependency resolution.

On a trusted machine with npm registry access:

```bash
cd web
npm install --package-lock-only --ignore-scripts --no-audit --no-fund
npm ci --ignore-scripts --no-audit --no-fund
npm run typecheck
npm run build
cd ..
```

Review `package-lock.json`, then commit it. Never hand-write or synthesize npm integrity hashes.

## 4. Run the release gate

```bash
make release-check
```

The gate checks migration numbering, clean migrations, Go formatting/vet/race tests, frontend lockfile/typecheck/build and production configuration sanity.

A failed gate blocks deployment.

## 5. Build the web image

`NEXT_PUBLIC_*` values are build-time values in Next.js. They must be embedded in the image that will be deployed.

```bash
export CHAT_WEB_IMAGE=chat-web:$(git rev-parse --short HEAD)

docker build \
  --build-arg NEXT_PUBLIC_API_BASE_URL="https://api.chat.example.com" \
  --build-arg NEXT_PUBLIC_VAPID_PUBLIC_KEY="${NEXT_PUBLIC_VAPID_PUBLIC_KEY:-}" \
  -t "$CHAT_WEB_IMAGE" \
  ./web
```

Write the exact image name into `.env.production` as `CHAT_WEB_IMAGE`.

## 6. Backup before an upgrade

For an existing environment, create and verify a backup before deploying new schema/application code.

See `DISASTER_RECOVERY.md` and `ops/backup/*`.

## 7. Validate the compose plan

Always pass the production env file explicitly so Compose interpolation sees the required values:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml config >/tmp/chat-compose.rendered.yml
```

Review the rendered plan for unexpected public ports. Do not share the rendered file because it may contain secrets.

## 8. Deploy

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build
```

Startup ordering is deliberate:

1. PostgreSQL becomes healthy.
2. `media-init` prepares the named media volume for the non-root API UID.
3. `migrate` obtains a PostgreSQL advisory lock and applies deterministic checksum-verified migrations.
4. API starts only if migrations completed successfully.
5. Web starts from the dependency-locked image.

## 9. Health / smoke checks

From the host:

```bash
curl --fail http://127.0.0.1:8080/health/live
curl --fail http://127.0.0.1:8080/health/ready
curl --fail http://127.0.0.1:3000/welcome >/dev/null
```

Then test through the real HTTPS reverse proxy:

- `/welcome`
- email magic-link request and completion
- Yandex ID if enabled
- onboarding
- direct message send/receive
- reconnect/offline retry
- group invitation preview + login + join
- photo upload/post
- report flow
- Trust Center session revoke/export

## 10. Load gate

Use the built-in load harness against the deployment hardware. Do not infer capacity from local development machines.

Record:

- concurrency
- RPS
- p50/p95/p99
- CPU/RAM
- WebSocket connection ceiling
- reconnect-storm behavior
- dropped realtime events

REST recovery must still recover acknowledged messages if realtime is intentionally interrupted.

## 11. Restore drill gate

Closed Alpha is not approved until a backup has been restored into an isolated target and verified. Record RPO/RTO and database/media consistency.

## 12. Reverse proxy requirements

The external proxy should provide:

- TLS 1.2+ / modern cipher configuration
- HTTP -> HTTPS redirect
- request/connection limits
- anti-DDoS/WAF where available
- WebSocket upgrade forwarding for `/api/v1/realtime`
- API origin routed to `127.0.0.1:8080`
- web origin routed to `127.0.0.1:3000`

Do not make PostgreSQL/Redis/NATS accessible through the proxy.

## 13. Rollback

Application rollback is allowed only after checking schema compatibility. Migrations are forward-tracked by checksum; never edit an already-applied `.up.sql` migration. Add a new corrective migration instead.

For an incident:

1. Stop acquisition/public write paths if needed.
2. Preserve logs/evidence.
3. Follow `SECURITY_INCIDENT_RUNBOOK.md` when security is involved.
4. Restore only from a verified backup if data restoration is actually required.
5. Record the incident and postmortem.

## 14. Release rule

Do not call the environment production-ready merely because containers are running. Alpha/Beta status is defined by the gates in `MASTER_PLAN.md` and `RELEASE_CHECKLIST.md`.
