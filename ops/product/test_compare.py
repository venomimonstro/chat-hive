import json
from datetime import datetime, timedelta, timezone
from pathlib import Path
import sys
import tempfile
import unittest
sys.path.insert(0, str(Path(__file__).resolve().parent))
from compare import compare

class ProductIterationTest(unittest.TestCase):
    def evaluate(self, baseline, after, *, metric="d7", days_apart=10):
        with tempfile.TemporaryDirectory() as folder:
            left, right = Path(folder)/"baseline.json", Path(folder)/"followup.json"
            now = datetime.now(timezone.utc) - timedelta(days=1)
            for path, values, dt in ((left, baseline, now-timedelta(days=days_apart)),
                                     (right, after, now)):
                path.write_text(json.dumps({"window_days": 30, "generated_at": dt.isoformat(), **values}))
            return compare(left, right, metric, 30, "Improved onboarding and message discovery",
                           "a"*40, "b"*40)

    def test_comparison_uses_eligible_denominators_not_total_signups(self):
        result = self.evaluate({"d7_retained":10,"d7_eligible":50}, {"d7_retained":24,"d7_eligible":60})
        self.assertEqual(result["outcome"], "improved")
        self.assertAlmostEqual(result["delta_percentage_points"], 20, places=3)
        self.assertEqual(len(result["baseline_sha256"]), 64)

    def test_regression_remains_visible_and_not_misreported(self):
        result = self.evaluate({"d7_retained":30,"d7_eligible":50}, {"d7_retained":15,"d7_eligible":50})
        self.assertEqual(result["outcome"], "not_improved")

    def test_invalid_cohort_and_too_early_followup_fail(self):
        with self.assertRaises(ValueError):
            self.evaluate({"d7_retained":40,"d7_eligible":35}, {"d7_retained":20,"d7_eligible":50})
        with self.assertRaises(ValueError):
            self.evaluate({"d7_retained":10,"d7_eligible":50}, {"d7_retained":20,"d7_eligible":50}, days_apart=2)
