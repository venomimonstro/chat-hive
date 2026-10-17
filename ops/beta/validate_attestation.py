#!/usr/bin/env python3
"""Check release-specific external evidence; does NOT cryptographically verify signers."""
from __future__ import annotations
from datetime import datetime, timedelta, timezone
import hashlib
import json
from pathlib import Path
import re
import sys

def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()

def validate(path: Path, pentest: Path, legal: Path, commit: str) -> None:
    data = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict) or data.get("schema") != "chat-beta-attestation-v1":
        raise ValueError("unknown beta attestation schema")
    if not re.fullmatch("[0-9a-f]{40}", commit) or data.get("release_commit") != commit:
        raise ValueError("attestation was not issued for this release commit")
    now = datetime.now(timezone.utc)
    for label, document, days in (("pentest",pentest,90),("legal",legal,180)):
        fields = data.get(label)
        if not isinstance(fields,dict):
            raise ValueError(f"missing {label} attestation")
        if document.stat().st_size < 256:
            raise ValueError(f"{label} document is empty or suspiciously short")
        if fields.get("sha256") != sha256(document):
            raise ValueError(f"{label} document SHA-256 differs from attested artifact")
        if len(str(fields.get("approved_by","")).strip()) < 5:
            raise ValueError(f"missing named {label} approver")
        try:
            signed = datetime.fromisoformat(str(fields["approved_at"]).replace("Z","+00:00"))
            if signed.tzinfo is None: raise ValueError()
            signed = signed.astimezone(timezone.utc)
        except (KeyError,ValueError,TypeError) as exc:
            raise ValueError(f"invalid {label} approval timestamp") from exc
        if not (now - timedelta(days=days) <= signed <= now):
            raise ValueError(f"{label} attestation is expired or future-dated")
        if fields.get("decision") != "approved":
            raise ValueError(f"{label} review was not approved")
    pen = data["pentest"]
    for severity in ("critical_open", "high_open"):
        if type(pen.get(severity)) is not int or pen[severity] != 0:
            raise ValueError("open Critical/High pentest findings block Beta")
    # Verify hashes and structure only. Identity/independence of approvers and
    # authenticity of documents are external legal/security responsibilities.

if __name__ == "__main__":
    try:
        if len(sys.argv) != 5:
            raise ValueError("usage: validate_attestation.py <json> <pentest> <legal> <git-sha>")
        validate(Path(sys.argv[1]),Path(sys.argv[2]),Path(sys.argv[3]),sys.argv[4])
    except (OSError,ValueError,TypeError,json.JSONDecodeError) as exc:
        print(f"BETA_ATTESTATION_BLOCKED: {exc}",file=sys.stderr)
        sys.exit(2)
    print("BETA_ATTESTATION_ARTIFACTS_VERIFIED")
