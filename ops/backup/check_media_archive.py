#!/usr/bin/env python3
"""Reject unsafe media archives before CHAT backup restore/extraction."""
from __future__ import annotations
import argparse
import os
import sys
import tarfile

def inspect(archive: str) -> tuple[int, int]:
    files: set[tuple[str, ...]] = set()
    paths: set[tuple[str, ...]] = set()
    total_bytes = 0
    entries = 0
    max_bytes = int(os.environ.get("CHAT_DR_MAX_MEDIA_BYTES", str(200 * 1024**3)))
    max_entries = int(os.environ.get("CHAT_DR_MAX_MEDIA_ENTRIES", "1000000"))
    with tarfile.open(archive, "r:gz") as stream:
        for item in stream:
            entries += 1
            if entries > max_entries:
                raise ValueError("media archive has too many entries")
            name = item.name
            if not name or name.startswith("/") or "\\" in name or "\x00" in name:
                raise ValueError(f"unsafe media path: {name!r}")
            parts = tuple(part for part in name.split("/") if part not in ("", "."))
            if ".." in parts:
                raise ValueError(f"path traversal in media archive: {name!r}")
            if not parts:
                if not item.isdir():
                    raise ValueError("archive root must be a directory")
                continue
            if not (item.isfile() or item.isdir()):
                raise ValueError(f"unsupported media member type: {name!r}")
            if parts in paths:
                raise ValueError(f"duplicate archive path: {name!r}")
            paths.add(parts)
            if item.isfile():
                files.add(parts)
                total_bytes += item.size
                if total_bytes > max_bytes:
                    raise ValueError("uncompressed media exceeds configured limit")
    for parts in paths:
        for offset in range(1, len(parts)):
            if parts[:offset] in files:
                raise ValueError("file is an ancestor of another media entry")
    return entries, total_bytes

def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("archive")
    args = parser.parse_args()
    try:
        entries, total_bytes = inspect(args.archive)
    except (OSError, tarfile.TarError, ValueError) as exc:
        print(f"UNSAFE_MEDIA_ARCHIVE: {exc}", file=sys.stderr)
        return 1
    print(f"MEDIA_ARCHIVE_OK entries={entries} bytes={total_bytes}")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
