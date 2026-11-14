"""Offline CLI regressions; no bot, Docker, capture or game connection is started."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone


SCRIPT = Path(__file__).with_name("auth_watchdog.py")


class AuthWatchdogTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.now = datetime.now(timezone.utc)

    def stamp(self, seconds=0):
        return (self.now + timedelta(seconds=seconds)).isoformat()

    def snapshot(self, phase="online", **character):
        return {
            "version": 1,
            "pid": 123,
            "updated_at": self.stamp(),
            "characters": [{"id": 1, "phase": phase, "reason": "", **character}],
        }

    def run_watchdog(self, *snapshots):
        paths = []
        for index, snapshot in enumerate(snapshots):
            path = Path(self.directory.name) / f"status-{index}.json"
            path.write_text(json.dumps(snapshot), encoding="utf-8")
            paths.append(str(path))
        return subprocess.run(
            [sys.executable, "-I", str(SCRIPT), *paths],
            capture_output=True, text=True, check=False,
        )

    def test_scheduled_block_keeps_watchdog_waiting(self):
        result = self.run_watchdog(self.snapshot("blocked", next_attempt_at=self.stamp(300)))
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_online_and_partial_recovery_have_different_exit_codes(self):
        self.assertEqual(self.run_watchdog(self.snapshot()).returncode, 1)
        self.assertEqual(self.run_watchdog(
            self.snapshot(), self.snapshot("backoff", next_attempt_at=self.stamp(30)),
        ).returncode, 0)

    def test_bounded_attempts_are_expected_waits(self):
        for phase in ("starting", "queued", "authenticating", "connecting"):
            with self.subTest(phase=phase):
                result = self.run_watchdog(self.snapshot(phase, attempt_deadline=self.stamp(40)))
                self.assertEqual(result.returncode, 0, result.stderr)

    def test_session_required_waits_for_operator_without_restarting(self):
        self.assertEqual(self.run_watchdog(self.snapshot("session_required")).returncode, 0)

    def test_deadline_errors_prevent_a_healthy_heartbeat_hiding_a_hang(self):
        for stamp in (-10, 120):
            with self.subTest(deadline=stamp):
                result = self.run_watchdog(self.snapshot("authenticating", attempt_deadline=self.stamp(stamp)))
                self.assertEqual(result.returncode, 2)

    def test_expired_or_unscheduled_retry_is_not_a_valid_wait(self):
        for phase in ("blocked", "backoff"):
            for character in ({}, {"next_attempt_at": self.stamp(-20)}):
                with self.subTest(phase=phase, character=character):
                    self.assertEqual(self.run_watchdog(self.snapshot(phase, **character)).returncode, 2)

    def test_retry_timer_transition_has_one_heartbeat_of_grace(self):
        for phase in ("blocked", "backoff"):
            with self.subTest(phase=phase):
                result = self.run_watchdog(self.snapshot(phase, next_attempt_at=self.stamp(-2)))
                self.assertEqual(result.returncode, 0, result.stderr)

    def test_stale_future_and_invalid_snapshots_fail_closed(self):
        snapshots = [
            {**self.snapshot(), "updated_at": self.stamp(-90)},
            {**self.snapshot(), "updated_at": self.stamp(90)},
            {**self.snapshot(), "version": 2},
            {**self.snapshot(), "characters": []},
            {**self.snapshot(), "pid": 0},
            self.snapshot("unknown"), self.snapshot("failed"), self.snapshot("stopped"),
            self.snapshot("queued"),
        ]
        for snapshot in snapshots:
            with self.subTest(snapshot=snapshot):
                self.assertEqual(self.run_watchdog(snapshot).returncode, 2)

    def test_invalid_process_dominates_a_different_process_waiting(self):
        result = self.run_watchdog(
            self.snapshot("blocked", next_attempt_at=self.stamp(300)),
            self.snapshot("failed"),
        )
        self.assertEqual(result.returncode, 2)

    def test_missing_and_malformed_files_report_only_safe_diagnostic(self):
        path = Path(self.directory.name) / "missing.json"
        result = subprocess.run([sys.executable, "-I", str(SCRIPT), str(path)], capture_output=True, text=True)
        self.assertEqual(result.returncode, 2)
        path.write_text("cookie-secret-value invalid json", encoding="utf-8")
        result = subprocess.run([sys.executable, "-I", str(SCRIPT), str(path)], capture_output=True, text=True)
        self.assertEqual(result.returncode, 2)
        self.assertNotIn("cookie-secret-value", result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
