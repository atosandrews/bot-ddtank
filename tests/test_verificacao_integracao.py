from types import SimpleNamespace

import pytest

from src import verificacao
from src.bot import Bot
from src.grupo import Grupo
from src.verificacao_ocr import ErroInfraestrutura, obter_ocr


def test_bot_individual_encaminha_para_controlador_sem_acao_de_batalha(monkeypatch):
    frame = object()
    driver = SimpleNamespace(capturar=lambda: frame)
    bot = Bot(driver, "solo")
    chamadas = []
    monkeypatch.setattr(verificacao, "tem_codigo", lambda f: f is frame)
    monkeypatch.setattr(verificacao, "tratar", lambda d, n: chamadas.append((d, n)))
    assert bot.passo()
    assert chamadas == [(driver, "solo")]


def test_grupo_encaminha_conta_detectada_e_interrompe_o_passo(monkeypatch):
    import src.grupo as modulo
    frame = object()
    driver = SimpleNamespace(capturar=lambda: frame)
    grupo = Grupo.__new__(Grupo)
    grupo.ultimo_codigo = 0
    grupo.contas = [SimpleNamespace(driver=driver, nome="convidada")]
    chamadas = []
    monkeypatch.setattr(modulo.time, "time", lambda: 10)
    monkeypatch.setattr(verificacao, "tem_codigo", lambda f: f is frame)
    monkeypatch.setattr(verificacao, "tratar", lambda d, n: chamadas.append((d, n)))
    grupo.passo()
    assert chamadas == [(driver, "convidada")]


def test_modelos_sao_carregados_uma_vez_por_processo(monkeypatch):
    from src import verificacao_ocr
    chamadas = []
    instancia = object()
    monkeypatch.setattr(verificacao_ocr, "OcrLocal", lambda: chamadas.append(True) or instancia)
    obter_ocr.cache_clear()
    try:
        assert obter_ocr() is instancia
        assert obter_ocr() is instancia
        assert chamadas == [True]
    finally:
        obter_ocr.cache_clear()


def test_expiracao_nao_e_apresentada_pelo_chamador_como_resolucao(monkeypatch):
    monkeypatch.setattr(verificacao, "resolver", lambda *args: verificacao.Resultado("EXPIRADO", 1))
    with pytest.raises(ErroInfraestrutura, match="expirou"):
        verificacao.tratar(object())
