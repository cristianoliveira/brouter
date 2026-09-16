import unittest
from datetime import datetime
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).parent))
from route import choose_target  # noqa: E402


class RouteTargetTests(unittest.TestCase):
    def test_work_url_is_personal_before_working_hours(self):
        self.assertEqual(
            choose_target(
                b"https://meetings.example.com/room",
                datetime(2026, 9, 14, 8, 59),
            ),
            b"personal",
        )

    def test_work_url_is_work_at_start_and_until_end(self):
        url = b"https://issues.example.com/ticket"
        self.assertEqual(choose_target(url, datetime(2026, 9, 14, 9, 0)), b"work")
        self.assertEqual(choose_target(url, datetime(2026, 9, 18, 16, 59)), b"work")

    def test_work_url_is_personal_at_end_and_on_weekend(self):
        url = b"https://meetings.example.com/room"
        self.assertEqual(choose_target(url, datetime(2026, 9, 14, 17, 0)), b"personal")
        self.assertEqual(choose_target(url, datetime(2026, 9, 19, 10, 0)), b"personal")

    def test_development_url_is_always_dev(self):
        url = b"http://localhost:3000/"
        self.assertEqual(choose_target(url, datetime(2026, 9, 14, 8, 0)), b"dev")
        self.assertEqual(choose_target(url, datetime(2026, 9, 14, 12, 0)), b"dev")

    def test_work_wins_overlap_during_hours(self):
        self.assertEqual(
            choose_target(
                b"https://meetings.example.com/localhost",
                datetime(2026, 9, 14, 9, 0),
            ),
            b"work",
        )

    def test_development_wins_overlap_outside_hours(self):
        self.assertEqual(
            choose_target(
                b"https://meetings.example.com/localhost",
                datetime(2026, 9, 14, 8, 59),
            ),
            b"dev",
        )

    def test_unmatched_url_defers_to_static_rules(self):
        self.assertEqual(
            choose_target(b"https://other.example.test/", datetime(2026, 9, 14, 12, 0)),
            b"@default",
        )


if __name__ == "__main__":
    unittest.main()
