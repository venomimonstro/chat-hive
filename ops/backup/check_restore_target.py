#!/usr/bin/env python3
"""Fail closed when a PostgreSQL restore target can be the live application DB."""
from __future__ import annotations
import os
import sys
from urllib.parse import unquote, urlsplit

def identity(value: str) -> tuple[str, int, str] | None:
    if not value:
        return None
    try:
        parts = urlsplit(value.strip())
        if parts.scheme not in ("postgres", "postgresql") or not parts.hostname:
            return None
        port = parts.port or 5432
        host = parts.hostname.lower().rstrip(".")
        if host in ("127.0.0.1", "::1"):
            host = "localhost"
        database = unquote(parts.path.removeprefix("/"))
        if not database:
            database = unquote(parts.username or "")
        if not database or not (0 < port <= 65535):
            return None
        return (host, port, database)
    except (ValueError, TypeError):
        return None

def check(target: str, live: list[str]) -> None:
    parsed = identity(target)
    if parsed is None:
        raise ValueError("RESTORE_DATABASE_URL must be an explicit postgres:// or postgresql:// URL")
    for source in live:
        if not source:
            continue
        reference = identity(source)
        # Inability to establish what the live database is must never be
        # treated as permission for a destructive restore.
        if reference is None:
            raise ValueError("live DATABASE_URL / CHAT_DATABASE_URL is not a comparable PostgreSQL URL")
        if parsed == reference:
            raise ValueError("restore target matches a configured live PostgreSQL database")

def main() -> int:
    try:
        target = os.environ.get("RESTORE_DATABASE_URL", "")
        check(target, [os.environ.get("DATABASE_URL", ""), os.environ.get("CHAT_DATABASE_URL", "")])
    except ValueError as exc:
        print(f"RESTORE_TARGET_BLOCKED: {exc}", file=sys.stderr)
        return 1
    print("RESTORE_TARGET_CHECK_OK")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
