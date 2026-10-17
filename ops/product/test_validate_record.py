import json
from datetime import datetime, timedelta, timezone
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
from compare import compare
from validate_record import validate

class ProductRecordValidationTest(unittest.TestCase):
    def test_verified_snapshots_required_and_tampering_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            before, after, record = (root/name for name in ("before.json","after.json","record.json"))
            now=datetime.now(timezone.utc) - timedelta(days=1)
            before.write_text(json.dumps({
                "generated_at":(now-timedelta(days=10)).isoformat(),
                "window_days":30,"d7_eligible":50,"d7_retained":10
            }))
            after.write_text(json.dumps({
                "generated_at":now.isoformat(),
                "window_days":30,"d7_eligible":70,"d7_retained":30
            }))
            item=compare(before,after,"d7",30,
                         "Documented experiment in conversation onboarding",
                         "a"*40,"b"*40)
            record.write_text(json.dumps(item))
            validate(record,before,after)
            item["delta_percentage_points"]=999
            record.write_text(json.dumps(item))
            with self.assertRaisesRegex(ValueError,"differs"):
                validate(record,before,after)
            item["delta_percentage_points"]=compare(before,after,"d7",30,
                item["change"],"a"*40,"b"*40)["delta_percentage_points"]
            record.write_text(json.dumps(item))
            after.write_text(after.read_text().replace('"d7_retained": 30','"d7_retained": 29'))
            with self.assertRaisesRegex(ValueError,"differs"):
                validate(record,before,after)
