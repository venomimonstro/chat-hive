"""Tests for deterministic recovery-point freshness handling."""
import sys
from datetime import datetime, timezone, timedelta
from pathlib import Path
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
from check_backup_age import age_seconds

class TestBackupAge(unittest.TestCase):
    def test_age_measured_from_utc_manifest(self):
        when = datetime(2026, 10, 17, 12, 0, 0, tzinfo=timezone.utc)
        with tempfile.TemporaryDirectory() as folder:
            manifest = Path(folder) / "manifest.txt"
            manifest.write_text("created_at=20261017T120000Z\nformat=chat-backup-v1\npostgres=postgres.dump\nmedia=none\n")
            self.assertEqual(age_seconds(manifest, when + timedelta(hours=2)), 7200)
            self.assertEqual(age_seconds(manifest, when - timedelta(minutes=3)), 0)
            with self.assertRaisesRegex(ValueError,"future"):
                age_seconds(manifest, when - timedelta(hours=1))

    def test_malformed_or_duplicate_timestamp_rejected(self):
        with tempfile.TemporaryDirectory() as folder:
            manifest = Path(folder) / "manifest.txt"
            for source in ["created_at=invalid\nformat=chat-backup-v1\n",
                "created_at=20261017T120000Z\ncreated_at=20261017T120000Z\nformat=chat-backup-v1\n",
                "created_at=20261017T120000Z\nformat=another-format\n"]:
                manifest.write_text(source)
                with self.assertRaises(ValueError):
                    age_seconds(manifest)
