# CHAT Disaster Recovery Runbook

## Principle

A backup is not considered valid until integrity verification and a restore drill have succeeded.

## Assets

1. PostgreSQL is the durable application source of truth.
2. `CHAT_MEDIA_ROOT` is the current local media store for the single-node phase. It will be replaced by versioned S3-compatible object storage before multi-node production.
3. Redis is ephemeral/cache/presence and is not required to reconstruct durable product state.
4. NATS/JetStream will carry async events; business writes must remain recoverable from PostgreSQL/outbox state.

## Backup

Required environment:

- `DATABASE_URL`
- optional `CHAT_MEDIA_ROOT` (defaults to `./var/media`, but the directory must exist)
- `CHAT_BACKUP_ALLOW_MISSING_MEDIA=1` only when you have verified this deployment has no media files
- optional `CHAT_BACKUP_DIR`

Run:

```sh
sh ops/backup/backup.sh
```

The job writes into a temporary directory, verifies the PostgreSQL archive, computes SHA-256 checksums and only then atomically renames the directory to its final timestamp.

## Scheduled daily backup on a systemd server

The scheduled service is **opt-in**: no server setting is modified by a Git
update or application restart. Install it on the actual host after configuring
a dedicated backup target and a restricted environment file. First make
`/etc/chat-hive/backup.env` (root-owned, chmod 600) containing
`DATABASE_URL`, an absolute `CHAT_BACKUP_DIR` and `CHAT_MEDIA_ROOT`.
Then run:

```bash
sudo bash ops/backup/install-timer.sh
sudo systemctl start chat-hive-backup.service
sudo systemctl status chat-hive-backup.service
systemctl list-timers --all chat-hive-backup.timer
```

The timer runs daily at 02:00 UTC with up to 15 minutes of jitter and
catches up missed events after reboot. The backup script now validates the
**promoted** artifact before declaring success. Failed jobs remain visible in
`journalctl -u chat-hive-backup.service`. Copy verified backups to
independent off-site/immutable storage through your storage provider; this
timer alone **does not** establish off-site copies or retention. Periodic
isolated restore drills and monitoring are still mandatory.

## Verification

```sh
sh ops/backup/verify.sh ./backups/<timestamp>
```

Verification checks the manifest, PostgreSQL custom archive and media tarball when present.

Media archive verification rejects traversal paths, symlinks, hardlinks,
device entries, duplicate paths and expanded size above 200 GiB by default
(adjust `CHAT_DR_MAX_MEDIA_BYTES` for larger installations).
Python 3 is required. The isolated restore drill also extracts media into a
temporary directory and includes extraction in reported `rto_seconds`.
Restoration also refuses matching `DATABASE_URL`/`RESTORE_DATABASE_URL` and
symlinks in the media destination's parent directories. Set a separate,
verified restore database and a dedicated media directory before enabling
`CHAT_ALLOW_RESTORE`.

Production restoration refuses absolute filesystem/system roots and requires
a dedicated absolute, non-symlink media directory.

## Restore drill

Preferred automated drill:

```sh
sh ops/backup/drill.sh ./backups/<timestamp>
```

The drill starts a temporary PostgreSQL container without published ports, mounts the backup read-only, verifies checksums, restores the dump, checks critical durable tables, extracts the media archive into an isolated temporary directory, and verifies each ready `media_objects` database record against its actual file size and SHA-256. A backup missing referenced media fails. It prints measured `rto_seconds` and removes the temporary resources on exit.

For a deeper manual drill with media restoration and an isolated application instance, never restore into the live database. Create an isolated PostgreSQL target and set:

```sh
export RESTORE_DATABASE_URL='postgres://.../chat_restore_test'
export RESTORE_MEDIA_ROOT='/tmp/chat-media-restore'
export CHAT_ALLOW_RESTORE='I_UNDERSTAND_THIS_REPLACES_TARGET_DATA'
sh ops/backup/restore.sh ./backups/<timestamp>
```

After restore, run application migrations in verification mode, start an isolated API instance and verify at minimum:

- `/health/ready` succeeds;
- a known test account/profile exists;
- chat history ordering and message counts are sane;
- post/media references resolve;
- moderation/admin audit tables are readable;
- append-only audit triggers remain installed;
- no production external notifications are enabled from the restore environment.

## Production policy before Public Beta

- Automatic daily database backups at minimum; increase frequency based on measured RPO.
- Backup storage credentials must differ from application runtime credentials.
- Backup destination must be outside the primary server/failure domain.
- Enable object versioning/immutability where the provider supports it.
- Quarterly restore drills before scale; monthly after business-critical adoption.
- Record `ops/backup/drill.sh` output, restore duration and resulting RPO/RTO in an incident/DR log.
- A successful backup job without a recent successful restore drill does not satisfy the release gate.

## Recovery-point recency gate

The isolated drill reports the backup age in seconds using the checksummed
`manifest.txt`. `CHAT_DRILL_MAX_BACKUP_AGE_SECONDS=0` only reports age;
a positive value fails the drill when it exceeds the allowed age.
The Closed Alpha gate uses a default budget of **86,400 seconds (24 hours)**
and accepts `CHAT_ALPHA_MAX_BACKUP_AGE_SECONDS` to impose a stricter policy.
This is a freshness check (the worst-case loss interval since the snapshot),
**not proof of transaction-level RPO**. A successful restore is still required.


The backup script rejects symlinked media roots and rejects layouts in which the
backup destination and the media root contain one another. This prevents
recursive self-archiving, disk exhaustion, and accidentally backing up files
outside the intended media directory. These checks run **before** pg_dump.
