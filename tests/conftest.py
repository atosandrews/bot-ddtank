import pytest


@pytest.fixture(autouse=True)
def diagnostico_isolado(monkeypatch, tmp_path):
    """O diagnóstico do captcha fica ligado no config; testes nunca gravam em logs/ reais."""
    from src import verificacao
    monkeypatch.setattr(verificacao, "PASTA_DIAGNOSTICO", tmp_path / "verificacao")
