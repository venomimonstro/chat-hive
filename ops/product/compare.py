#!/usr/bin/env python3
"""Compare genuine CHAT Product Metrics snapshots after one documented change."""
from __future__ import annotations
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import sys

METRICS = {
    "d1": ("d1_retained", "d1_eligible", 1),
    "d7": ("d7_retained", "d7_eligible", 7),
}

def load(path: Path) -> tuple[dict, bytes, datetime]:
    raw = path.read_bytes()
    data = json.loads(raw)
    if not isinstance(data, dict):
        raise ValueError("snapshot must be a JSON object")
    try:
        ts = datetime.fromisoformat(data["generated_at"].replace("Z", "+00:00"))
        if ts.tzinfo is None:
            raise ValueError("timestamp must be timezone-aware")
    except (KeyError, AttributeError, ValueError) as exc:
        raise ValueError("snapshot missing valid generated_at") from exc
    return data, raw, ts.astimezone(timezone.utc)

def compare(before: Path, after: Path, metric: str, min_eligible: int, change: str,
            commit_before: str, commit_after: str) -> dict:
    if not change.strip() or len(change.strip()) < 20:
        raise ValueError("describe the actual released product change in at least 20 characters")
    for commit in (commit_before, commit_after):
        if not re.fullmatch("[0-9a-f]{40}", commit):
            raise ValueError("snapshot deployment commits must be full 40-digit Git SHAs")
    if commit_before == commit_after:
        raise ValueError("product iteration must compare two different deployed revisions")
    if min_eligible < 30:
        raise ValueError("at least 30 eligible users are required per cohort")
    b, braw, bt = load(before)
    a, araw, at = load(after)
    retained_key, eligible_key, minimum_days = METRICS[metric]
    if b.get("window_days") != a.get("window_days") or b.get("window_days") not in (7, 30, 90):
        raise ValueError("metrics windows must be equal and supported")
    if (at - bt).total_seconds() < minimum_days * 86400:
        raise ValueError("follow-up snapshot too close to baseline for selected retention horizon")
    if at > datetime.now(timezone.utc):
        raise ValueError("follow-up metrics timestamp lies in the future")
    observations = []
    for point in (b, a):
        eligible, retained = point.get(eligible_key), point.get(retained_key)
        if not all(isinstance(x, int) and not isinstance(x, bool) for x in (eligible, retained)):
            raise ValueError("eligible and retained counts must be integers")
        if eligible < min_eligible or retained < 0 or retained > eligible:
            raise ValueError("retention counts are insufficient or inconsistent")
        observations.append((retained, eligible))
    (bret, belig), (aret, aelig) = observations
    before_rate, after_rate = bret / belig, aret / aelig
    delta_pp = (after_rate - before_rate) * 100
    # The evaluator is intentionally descriptive: comparing overlapping
    # rolling windows does NOT prove the change caused the difference.
    result = {
        "schema": "chat-product-iteration-v1",
        "metric": metric, "window_days": b["window_days"],
        "change": change.strip(), "commit_before": commit_before,
        "commit_after": commit_after,
        "baseline_generated_at": bt.isoformat(),
        "followup_generated_at": at.isoformat(),
        "baseline_sha256": hashlib.sha256(braw).hexdigest(),
        "followup_sha256": hashlib.sha256(araw).hexdigest(),
        "baseline_retained": bret, "baseline_eligible": belig,
        "followup_retained": aret, "followup_eligible": aelig,
        "baseline_rate": round(before_rate, 6),
        "followup_rate": round(after_rate, 6),
        "delta_percentage_points": round(delta_pp, 4),
        "outcome": "improved" if delta_pp > 0 else "not_improved",
        "caveat": "Descriptive rolling cohort change; causality is not established."
    }
    return result

def main() -> int:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("baseline", type=Path)
    p.add_argument("followup", type=Path)
    p.add_argument("--metric", choices=tuple(METRICS), required=True)
    p.add_argument("--change", required=True)
    p.add_argument("--commit-before", required=True)
    p.add_argument("--commit-after", required=True)
    p.add_argument("--min-eligible", type=int, default=30)
    p.add_argument("--out", type=Path, required=True)
    args = p.parse_args()
    try:
        result = compare(args.baseline, args.followup, args.metric, args.min_eligible,
                         args.change, args.commit_before, args.commit_after)
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    except (OSError, ValueError, KeyError, json.JSONDecodeError) as exc:
        print(f"PRODUCT_ITERATION_BLOCKED: {exc}", file=sys.stderr)
        return 2
    print(f"Product iteration outcome={result['outcome']} delta_pp={result['delta_percentage_points']:+.2f}")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
