#!/usr/bin/env python3
"""Capture privacy-minimized owner Product Metrics to a private local JSON file."""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import ssl
import sys
import tempfile
from urllib.parse import urlsplit
from urllib.request import Request, build_opener, HTTPRedirectHandler, HTTPSHandler

class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        # Never forward an administrative Bearer token across redirects.
        raise ValueError("Metrics endpoint unexpectedly redirected")

def validate_endpoint(value: str) -> str:
    from urllib.parse import urlsplit
    parsed = urlsplit(value)
    if (parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password
            or parsed.query or parsed.fragment or parsed.path not in ("", "/")):
        raise ValueError("CHAT_ALPHA_URL must be a bare HTTPS origin without credentials")
    return value.rstrip("/")

def check_metrics(data: object, days: int) -> dict:
    if not isinstance(data, dict) or data.get("window_days") != days:
        raise ValueError("API returned wrong Product Metrics window")
    required = ("registrations", "active_users", "d1_eligible", "d1_retained",
                "d7_eligible", "d7_retained")
    for name in required:
        value = data.get(name)
        if type(value) is not int or value < 0:
            raise ValueError(f"invalid {name} in Product Metrics response")
    for prefix in ("d1", "d7"):
        if data[prefix + "_retained"] > data[prefix + "_eligible"]:
            raise ValueError("retained cohort exceeds its eligible population")
    if not isinstance(data.get("generated_at"), str):
        raise ValueError("missing Product Metrics generation timestamp")
    return data

def save_private(target: Path, data: dict) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    # Refuse symlinked or existing output targets; never clobber baseline data.
    if target.is_symlink() or target.exists():
        raise ValueError("Metrics output already exists or is a symlink")
    fd, name = tempfile.mkstemp(prefix=".product-metrics-", dir=str(target.parent))
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as file:
            os.fchmod(file.fileno(), 0o600)
            json.dump(data, file, ensure_ascii=False, indent=2)
            file.write("\n")
            file.flush()
            os.fsync(file.fileno())
        os.link(name, target)
    finally:
        if os.path.exists(name):
            os.unlink(name)

def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--days", type=int, choices=(7,30,90), default=30)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    try:
        origin = validate_endpoint(os.environ.get("CHAT_ALPHA_URL", ""))
        token = os.environ.get("CHAT_PRODUCT_ACCESS_TOKEN", "")
        if len(token) < 16:
            raise ValueError("CHAT_PRODUCT_ACCESS_TOKEN owner passkey session is required")
        url = f"{origin}/api/v1/admin/product/metrics?days={args.days}"
        request = Request(url, headers={"Authorization": "Bearer " + token,
                                        "Accept": "application/json", "Cache-Control": "no-store"})
        opener = build_opener(NoRedirect, HTTPSHandler(context=ssl.create_default_context()))
        with opener.open(request, timeout=20) as response:
            if response.status != 200:
                raise ValueError(f"Product Metrics status={response.status}")
            body = response.read(65537)
        if len(body) > 65536:
            raise ValueError("oversized Product Metrics response")
        result = check_metrics(json.loads(body), args.days)
        save_private(args.out, result)
    except Exception as exc:
        # Never dump credential-bearing URLs, headers or exceptions containing
        # the private owner token into CI/console output.
        print(f"METRICS_CAPTURE_BLOCKED: {type(exc).__name__}; verify HTTPS endpoint, owner session, output path", file=sys.stderr)
        return 2
    print(f"Saved private Product Metrics snapshot: {args.out} (mode 0600)")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
