# CHAT Hive

CHAT is a mobile-first social messenger focused on fast communication, social discovery, creator growth and reliable operation on poor networks.

## Status (repository code, not deployment certification)

- Product functionality through Sprint 24 is marked DONE in `docs/MASTER_PLAN.md`.
- Sprints 25–30 are **in progress** and require real-host load tests, security review, restore drills and Alpha/Beta evidence.
- Backend: Go modular monolith; frontend: Next.js + TypeScript PWA; database: PostgreSQL.
- Realtime: WebSocket + PostgreSQL LISTEN/NOTIFY, with REST recovery.
- Redis/NATS are provisioned for future scaling; do not claim they carry production chat traffic.
- No CI/GitHub Actions: changes land in `main`, and `bash ops/release/check.sh` is the mandatory manual release gate.
- A real `web/package-lock.json` must be generated with npm before frontend Docker builds or release checks can pass.

## Local development

The repository is being bootstrapped in Sprint 00–01. See `docs/MASTER_PLAN.md` for the authoritative sprint status and acceptance criteria.
