"""Offline consistency checks for media restored from a database backup."""
import hashlib
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
from check_restored_media import validate

class TestRestoreConsistency(unittest.TestCase):
    def test_files_match_database_digests(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder) / "media"
            key = "ab/" + "ab" + "0" * 30 + ".jpg"
            file = root / key
            file.parent.mkdir(parents=True)
            file.write_bytes(b"hello")
            rows = Path(folder) / "rows.txt"
            digest = hashlib.sha256(b"hello").hexdigest()
            rows.write_text(f"{key}|5|{digest}\n", encoding="utf-8")
            self.assertEqual(validate(root, rows), 1)
            file.write_bytes(b"world")
            with self.assertRaisesRegex(ValueError, "checksum"):
                validate(root, rows)

    def test_missing_media_and_unsafe_keys_fail(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder) / "media"
            root.mkdir()
            rows = Path(folder) / "rows.txt"
            rows.write_text("../outside.jpg|4|" + "0" * 64 + "\n")
            with self.assertRaisesRegex(ValueError, "unsafe storage_key"):
                validate(root, rows)
            rows.write_text("aa/" + "a" * 32 + ".jpg|4|" + "0" * 64 + "\n")
            with self.assertRaisesRegex(ValueError, "missing"):
                validate(root, rows)
