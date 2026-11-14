"""Execute only supervisor loops with fake commands; startup/game code is never run.

Uses a native POSIX shell on Linux, or the existing golang:1.26 Docker image on
Windows. Docker runs without networking or mounts. No credentials are read.
"""
from pathlib import Path
import re
import shutil
import subprocess
import sys
import unittest


HEADLESS = Path(__file__).resolve().parents[2]
SCRIPTS = {"formigueiro": HEADLESS / "codigo/run/bot-all.sh",
           "pintinho": HEADLESS / "pintinho/bot-pintinho.sh"}


def shell_command():
    if sys.platform != "win32" and shutil.which("sh"):
        return ["sh"]
    if shutil.which("docker"):
        return ["docker", "run", "--rm", "-i", "--network", "none", "golang:1.26", "sh"]
    raise unittest.SkipTest("requires POSIX sh or Docker with golang:1.26 already available")


def run_loop(variant, scenario, iterations=3):
    source = SCRIPTS[variant].read_text(encoding="utf-8")
    match = re.search(r"(?m)^while sleep [0-9]+; do$", source)
    if not match:
        raise AssertionError("supervisor loop entry not found")
    # Definitions replace every external effect used by the loop. The real
    # startup, compilation, cleanup, capture and login commands stay excluded.
    harness = r'''
set -u
iteration=0
sleep() { iteration=$((iteration + 1)); [ "$iteration" -le "$ITERATIONS" ]; }
date() { echo $((1000 + iteration * 60)); }
pgrep() { [ "$SCENARIO" != dead ]; }
python3() {
  case "$*" in
    *auth_watchdog.py*)
      case "$SCENARIO" in
        stale) return 2 ;;
        helper_missing) return 127 ;;
        blocked|dead|queue_full|control_dead) return 0 ;;
        guests_blocked)
          case "$*" in *auth-guests.json*) return 0 ;; *) return 1 ;; esac ;;
        recovered) [ "$iteration" -gt 1 ] && return 1; return 0 ;;
        *) return 1 ;;
      esac ;;
    *) return 0 ;;
  esac
}
conectados() {
  case "$SCENARIO" in
    blocked|guests_blocked|healthy_missing) echo "1" ;;
    *) echo "1 2 3 4" ;;
  esac
}
partidas() { echo "unchanged"; }
coletar() { :; }
controle_saude() { [ "$SCENARIO" != control_dead ]; }
fila_cheia() { [ "$SCENARIO" = queue_full ]; }
em_partida() { return 1; }
esperando_sala() { return 1; }
sem_confirmacao() { echo 0; }
progresso() { echo "unchanged"; }
renovar() { :; }
SEM_PROGRESSO=420
SEM_PARTIDA=300
PERSONAGENS=4
RENOVAR_COOKIE=999999
renovado=1000
faltando=0
ultimo=unchanged
desde=1000
sala=0
partida=1000
cheia=0
confirmacoes=0
confirmacoes_hora=1000
if [ "$SCENARIO" = recovered ]; then
  faltando=99; sala=99; desde=0; partida=0
fi
'''
    program = f"SCENARIO={scenario}\nITERATIONS={iterations}\n" + harness + source[match.start():] + "\necho FINISHED\n"
    # Bytes prevent Windows subprocess text mode from changing LF into CRLF.
    command = shell_command()
    result = subprocess.run(command, input=program.encode("utf-8"), capture_output=True, timeout=30)
    return subprocess.CompletedProcess(command, result.returncode, result.stdout.decode("utf-8"), result.stderr.decode("utf-8"))


class SupervisorAuthTests(unittest.TestCase):
    def test_blocked_login_does_not_restart_either_group(self):
        for variant in SCRIPTS:
            with self.subTest(variant=variant):
                result = run_loop(variant, "blocked")
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn("FINISHED", result.stdout)

    def test_blocked_guests_are_aggregated_with_online_main(self):
        result = run_loop("pintinho", "guests_blocked")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_dead_process_overrides_blocked_status(self):
        for variant in SCRIPTS:
            with self.subTest(variant=variant):
                self.assertEqual(run_loop(variant, "dead").returncode, 1)

    def test_stale_auth_heartbeat_is_an_operational_failure(self):
        for variant in SCRIPTS:
            with self.subTest(variant=variant):
                result = run_loop(variant, "stale", iterations=1)
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)

    def test_normal_missing_members_still_trigger_recovery(self):
        for variant in SCRIPTS:
            with self.subTest(variant=variant):
                self.assertEqual(run_loop(variant, "healthy_missing").returncode, 1)

    def test_recovery_discards_old_stall_counters(self):
        for variant in SCRIPTS:
            with self.subTest(variant=variant):
                result = run_loop(variant, "recovered", iterations=2)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn("FINISHED", result.stdout)

    def test_query_queue_exhaustion_is_not_hidden_by_blocked_login(self):
        self.assertEqual(run_loop("pintinho", "queue_full", iterations=1).returncode, 1)

    def test_blocked_login_does_not_hide_unhealthy_relay_control(self):
        for variant in SCRIPTS:
            with self.subTest(variant=variant):
                result = run_loop(variant, "control_dead", iterations=1)
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)

    def test_unexpected_watchdog_exit_is_an_operational_failure(self):
        for variant in SCRIPTS:
            with self.subTest(variant=variant):
                result = run_loop(variant, "helper_missing", iterations=1)
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
