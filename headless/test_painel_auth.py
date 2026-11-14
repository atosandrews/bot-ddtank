"""Offline panel regressions. No Docker, game or desktop access."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from datetime import datetime, timezone


def load_panel():
    spec = importlib.util.spec_from_file_location("panel_under_test", Path(__file__).with_name("painel.py"))
    module = importlib.util.module_from_spec(spec)
    with patch("subprocess.run", side_effect=AssertionError("import must not invoke Docker")):
        spec.loader.exec_module(module)
    return module


class PanelAuthTests(unittest.TestCase):
    def setUp(self):
        self.panel = load_panel()
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.panel.PASTA = Path(self.directory.name)
        self.panel.FARM = self.panel.PASTA / "farm.json"

    def render(self, ready=None, events=None, auth=None, sessions=None):
        timestamp = datetime.now(timezone.utc).isoformat()
        old_login = {"character_id": 1, "msg": "headless game login confirmed", "time": timestamp}
        with patch.object(self.panel, "eventos", return_value=("running", events or [old_login])), \
             patch.object(self.panel, "formigueiro", return_value=ready or {}), \
             patch.object(self.panel, "sessoes_prontas", return_value=sessions if sessions is not None else {ident: ident for ident in (ready or {})}, create=True), \
             patch.object(self.panel, "nomes_itens", return_value={}), \
             patch.object(self.panel, "personagens", return_value=[{"id": "1", "name": "offline", "website_id": "2", "upstream": "offline:1"}]), \
             patch.object(self.panel, "autenticacao", return_value=auth or {}, create=True):
            self.panel.main()
        return (self.panel.PASTA / "painel.html").read_text(encoding="utf-8")

    def test_import_does_not_start_panel_or_contact_docker(self):
        self.assertTrue(callable(self.panel.main))

    def test_historical_login_is_not_counted_online(self):
        output = self.render()
        self.assertIn("Personagens online: 0 de 1", output)

    def test_current_ready_session_overrides_historical_error(self):
        output = self.render(ready={1: {"id": 1}}, events=[{"character_id": 1, "msg": "old error", "level": "ERROR", "error": "old block"}])
        self.assertIn("Status: online", output)
        self.assertIn("Personagens online: 1 de 1", output)

    def test_blocked_login_and_next_attempt_are_visible(self):
        output = self.render(auth={1: {"phase": "blocked", "next_attempt_at": "2026-11-13T18:30:00Z"}})
        self.assertIn("login bloqueado", output)
        self.assertIn("Próxima tentativa:", output)
        self.assertIn("Personagens online: 0 de 1", output)

    def test_expired_session_is_visible(self):
        self.assertIn("aguardando sessão", self.render(auth={1: {"phase": "session_required"}}))

    def test_connecting_is_visible(self):
        self.assertIn("reconectando", self.render(auth={1: {"phase": "connecting"}}))

    def test_ready_session_remains_online_when_projection_is_unavailable(self):
        with patch.object(self.panel, "acumular", side_effect=AssertionError("missing projection must not reset farm")):
            output = self.render(sessions={1: 77})
        self.assertIn("Status: online", output)
        self.assertIn("Personagens online: 1 de 1", output)

    def test_projection_is_not_queried_for_non_ready_sessions(self):
        responses = [type("Reply", (), {"stdout": json.dumps({"value": [{"session": 1, "character_id": 11, "game_ready": False}, {"session": 2, "character_id": 22, "game_ready": True}]})}),
                     type("Reply", (), {"stdout": json.dumps({"value": {"id": 22}})})]
        with patch.object(self.panel.subprocess, "run", side_effect=responses) as command:
            self.assertEqual(self.panel.formigueiro(), {22: {"id": 22}})
        self.assertEqual(command.call_count, 2)

    def test_readiness_maps_character_identity_without_lua_projection(self):
        reply = type("Reply", (), {"stdout": json.dumps({"value": [
            {"session": 7, "character_id": 123, "game_ready": True},
            {"session": 8, "character_id": 22, "game_ready": False},
            {"session": 9, "game_ready": True}]})})
        with patch.object(self.panel.subprocess, "run", return_value=reply):
            self.assertEqual(self.panel.sessoes_prontas(), {123: 7})

    def test_stale_auth_snapshot_is_ignored(self):
        reply = type("Reply", (), {"stdout": json.dumps({"version": 1, "updated_at": "2020-01-01T00:00:00Z", "characters": [{"id": 1, "phase": "online"}]}), "returncode": 0})
        with patch.object(self.panel.subprocess, "run", return_value=reply):
            self.assertEqual(self.panel.autenticacao(), {})


if __name__ == "__main__":
    unittest.main()
