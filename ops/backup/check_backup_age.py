#!/usr/bin/env python3
"""Validate recovery point recency from a checksummed CHAT backup manifest."""
from __future__ import annotations
import argparse
from datetime import datetime, timezone
from pathlib import Path
import re
import sys

def age_seconds(manifest: Path, now: datetime | None = None) -> int:
    entries = {}
    for line in manifest.read_text(encoding="utf-8").splitlines():
        if "=" not in line:
            raise ValueError("invalid backup manifest entry")
        key, value = line.split("=", 1)
        if key in entries:
            raise ValueError("duplicate backup manifest key")
        entries[key] = value
    if entries.get("format") != "chat-backup-v1":
        raise ValueError("unknown backup format")
    value = entries.get("created_at", "")
    if not re.fullmatch(r"\d{8}T\d{6}Z", value):
        raise ValueError("invalid backup timestamp")
    created = datetime.strptime(value, "%Y%m%dT%H%M%SZ").replace(tzinfo=timezone.utc)
    if now is None:
        now = datetime.now(timezone.utc)
    delta = (now - created).total_seconds()
    if delta < -300:
        raise ValueError("backup timestamp lies in the future")
    return max(0, int(delta))

def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--max-age", type=int, default=0, help="maximum age in seconds, 0 to report only")
    args = parser.parse_args()
    try:
        if args.max_age < 0:
            raise ValueError("max-age must not be negative")
        age = age_seconds(args.manifest)
        print(f"BACKUP_AGE_SECONDS={age} MAX_ALLOWED={args.max_age}")
        if args.max_age > 0 and age > args.max_age:
            raise ValueError(f"backup exceeds recovery-point freshness budget: {age} > {args.max_age}")
    except (OSError, UnicodeError, ValueError) as exc:
        print(f"BACKUP_RECENCY_FAILED: {exc}", file=sys.stderr)
        return 1
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
