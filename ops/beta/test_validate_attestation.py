import hashlib
import json
from datetime import datetime,timezone
from pathlib import Path
import sys
import tempfile
import unittest
sys.path.insert(0,str(Path(__file__).resolve().parent))
from validate_attestation import validate

class AttestationTest(unittest.TestCase):
    def test_valid_artifacts_and_tampering(self):
        with tempfile.TemporaryDirectory() as folder:
            base=Path(folder)
            pentest=base/"pentest.pdf"
            legal=base/"legal.txt"
            pentest.write_bytes(b"%PDF-1.4\n"+b"p"*1024)
            legal.write_bytes(b"Named legal approval " + b"l"*300)
            now=datetime.now(timezone.utc).isoformat()
            data={
                "schema":"chat-beta-attestation-v1",
                "release_commit":"a"*40,
                "pentest":{"sha256":hashlib.sha256(pentest.read_bytes()).hexdigest(),
                           "decision":"approved","approved_by":"Independent Security Company",
                           "approved_at":now,"critical_open":0,"high_open":0},
                "legal":{"sha256":hashlib.sha256(legal.read_bytes()).hexdigest(),
                         "decision":"approved","approved_by":"Legal Counsel",
                         "approved_at":now}
            }
            record=base/"attestation.json"
            record.write_text(json.dumps(data))
            validate(record,pentest,legal,"a"*40)
            legal.write_bytes(legal.read_bytes()+b"tampering")
            with self.assertRaisesRegex(ValueError,"differs"):
                validate(record,pentest,legal,"a"*40)
            with self.assertRaisesRegex(ValueError,"release"):
                validate(record,pentest,legal,"b"*40)

    def test_open_high_findings_fail(self):
        with tempfile.TemporaryDirectory() as folder:
            base=Path(folder)
            report=base/"p.pdf"; report.write_bytes(b"x"*500)
            legal=base/"l.txt"; legal.write_bytes(b"y"*500)
            now=datetime.now(timezone.utc).isoformat()
            fields=lambda data: {"sha256":hashlib.sha256(data).hexdigest(),
                                  "decision":"approved","approved_by":"Named Reviewer",
                                  "approved_at":now}
            attestation=base/"approval.json"
            attestation.write_text(json.dumps({
                "schema":"chat-beta-attestation-v1","release_commit":"a"*40,
                "pentest":{**fields(report.read_bytes()),"critical_open":0,"high_open":1},
                "legal":fields(legal.read_bytes())
            }))
            with self.assertRaisesRegex(ValueError,"findings"):
                validate(attestation,report,legal,"a"*40)
