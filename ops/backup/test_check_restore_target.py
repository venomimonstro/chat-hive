"""Dangerous restore-target equivalences that must be detected offline."""
import sys
from pathlib import Path
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
from check_restore_target import check, identity

class RestoreTargetTest(unittest.TestCase):
    def test_matches_with_different_users_schemes_passwords_and_ssl_options(self):
        live = "postgres://app:old@db.internal:5432/chat?sslmode=require"
        for target in (
            "postgresql://backup:new@DB.INTERNAL/chat?sslmode=disable",
            "postgres://postgres:any@db.internal.:5432/chat",
        ):
            with self.subTest(target=target), self.assertRaisesRegex(ValueError, "matches"):
                check(target, [live])

    def test_production_chat_database_url_is_protected(self):
        with self.assertRaisesRegex(ValueError, "matches"):
            check("postgres://restore@primary:5432/chat", ["", "postgres://app@primary/chat"])

    def test_rejects_invalid_target_and_unknown_live_identity(self):
        with self.assertRaises(ValueError):
            check("host=localhost dbname=chat", [])
        with self.assertRaises(ValueError):
            check("postgres://user@restore/db", ["host=primary dbname=chat"])

    def test_separate_database_is_accepted(self):
        check("postgresql://restore@db.internal/chat-restore", ["postgres://app@db.internal/chat"])
        self.assertEqual(identity("postgresql://u@[::1]/demo"), ("localhost", 5432, "demo"))
