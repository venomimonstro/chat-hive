import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
sys.path.insert(0, str(Path(__file__).resolve().parent))
from capture import validate_endpoint, check_metrics, save_private

class CaptureTest(unittest.TestCase):
    def test_https_origin_validation(self):
        self.assertEqual(validate_endpoint("https://chat.example.ru/"), "https://chat.example.ru")
        for value in ("http://chat.example.ru","https://token@chat.example.ru",
                      "https://chat.example.ru/other","https://chat.example.ru/?next=x",""):
            with self.subTest(value=value), self.assertRaises(ValueError):
                validate_endpoint(value)

    def test_private_atomic_snapshot_and_no_overwrite(self):
        with tempfile.TemporaryDirectory() as folder:
            target=Path(folder)/"metrics.json"
            value={"window_days":30,"registrations":50,"active_users":12,
                   "d1_eligible":40,"d1_retained":10,"d7_eligible":30,"d7_retained":5,
                   "generated_at":"2026-10-17T00:00:00Z"}
            check_metrics(value,30)
            save_private(target,value)
            self.assertEqual(target.stat().st_mode & 0o777, 0o600)
            self.assertEqual(json.loads(target.read_text()),value)
            with self.assertRaises(ValueError):
                save_private(target, value)
            value["d1_retained"]=41
            with self.assertRaises(ValueError):
                check_metrics(value,30)
