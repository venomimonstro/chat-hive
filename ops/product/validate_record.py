#!/usr/bin/env python3
"""Require a measured, documented product change before widening Closed Alpha."""
import json
from pathlib import Path
import re
import sys

def validate(path: Path) -> None:
    record = json.loads(path.read_text(encoding="utf-8"))
    if record.get("schema") != "chat-product-iteration-v1":
        raise ValueError("unsupported product iteration record")
    if record.get("outcome") != "improved":
        raise ValueError("product metrics have not improved")
    if record.get("metric") not in ("d1", "d7"):
        raise ValueError("D1/D7 retention metric required")
    if any(not isinstance(record.get(field), int) or record[field] < 30
           for field in ("baseline_eligible", "followup_eligible")):
        raise ValueError("cohort too small for beta evidence")
    if not (isinstance(record.get("delta_percentage_points"), (int,float))
            and record["delta_percentage_points"] > 0):
        raise ValueError("improvement missing")
    for field in ("baseline_sha256", "followup_sha256", "commit_before", "commit_after"):
        size = 64 if field.endswith("sha256") else 40
        if not re.fullmatch("[0-9a-f]{"+str(size)+"}", record.get(field,"")):
            raise ValueError("snapshot fingerprint or commit SHA invalid")

if __name__ == "__main__":
    try:
        if len(sys.argv) != 2: raise ValueError("usage: validate_record.py <record.json>")
        validate(Path(sys.argv[1]))
    except (OSError, ValueError, TypeError, json.JSONDecodeError) as exc:
        print(f"BETA_PRODUCT_GATE_BLOCKED: {exc}", file=sys.stderr)
        sys.exit(2)
    print("BETA_PRODUCT_RECORD_OK")
