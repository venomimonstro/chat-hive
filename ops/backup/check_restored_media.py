#!/usr/bin/env python3
"""Check restored files against durable media_objects rows, without networking."""
from __future__ import annotations

import argparse
import hashlib
from pathlib import Path
import re
import sys

KEY = re.compile(r"([a-f0-9]{2})/([a-f0-9]{32})\.(jpg|png)\Z")

def validate(root: Path, metadata: Path) -> int:
    if not root.is_dir():
        raise ValueError("restored media directory missing")
    seen = set()
    matched = 0
    for line_number, line in enumerate(metadata.read_text(encoding="utf-8").splitlines(), 1):
        if not line:
            continue
        parts = line.split("|")
        if len(parts) != 3:
            raise ValueError(f"invalid media metadata row {line_number}")
        key, size, digest = parts
        match = KEY.fullmatch(key)
        if match is None or match.group(1) != match.group(2)[:2]:
            raise ValueError(f"unsafe storage_key in row {line_number}")
        if key in seen:
            raise ValueError(f"duplicate storage_key in row {line_number}")
        seen.add(key)
        try:
            expected_size = int(size)
        except ValueError as exc:
            raise ValueError(f"invalid size in row {line_number}") from exc
        if expected_size < 0 or not re.fullmatch(r"[0-9a-f]{64}", digest):
            raise ValueError(f"invalid digest/size in row {line_number}")
        file = root / key
        if file.parent.is_symlink() or file.is_symlink() or not file.is_file():
            raise ValueError(f"restored media missing or unsafe: {key}")
        actual_size = 0
        sha = hashlib.sha256()
        with file.open("rb") as stream:
            for chunk in iter(lambda: stream.read(1024 * 1024), b""):
                sha.update(chunk)
                actual_size += len(chunk)
        if actual_size != expected_size or sha.hexdigest() != digest:
            raise ValueError(f"restored media checksum/size mismatch: {key}")
        matched += 1
    return matched

def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("media_root", type=Path)
    parser.add_argument("media_metadata", type=Path)
    args = parser.parse_args()
    try:
        count = validate(args.media_root, args.media_metadata)
    except (OSError, UnicodeError, ValueError) as exc:
        print(f"RESTORED_MEDIA_INVALID: {exc}", file=sys.stderr)
        return 1
    print(f"RESTORED_MEDIA_OK referenced_files={count}")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
