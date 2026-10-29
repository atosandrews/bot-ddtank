import pytest


@pytest.fixture(autouse=True)
def logs_isolados(monkeypatch, tmp_path):
    """Diagnósticos ficam ligados no config; testes nunca gravam na pasta logs/ real."""
    from src import batalha, registro, verificacao
    monkeypatch.setattr(verificacao, "PASTA_DIAGNOSTICO", tmp_path / "verificacao")
    monkeypatch.setattr(batalha, "PASTA_TIROS", tmp_path / "tiros")
    monkeypatch.setattr(registro, "LOGS", tmp_path)
