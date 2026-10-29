import pytest

from src.drivers import tela


@pytest.fixture
def ambiente(monkeypatch):
    import win32gui
    estado = {"foco": 10, "janela": 10, "aberta": True,
              "regiao": {"left": 100, "top": 50, "width": 600, "height": 400}}
    d = tela.DriverTela(hwnd=10)
    d.regiao = dict(estado["regiao"])
    monkeypatch.setattr(d, "_atualizar_regiao", lambda: setattr(d, "regiao", dict(estado["regiao"])))
    monkeypatch.setattr(win32gui, "IsWindow", lambda h: estado["aberta"])
    monkeypatch.setattr(win32gui, "GetForegroundWindow", lambda: estado["foco"])
    monkeypatch.setattr(win32gui, "WindowFromPoint", lambda p: estado["janela"])
    monkeypatch.setattr(win32gui, "GetAncestor", lambda h, flag: h)
    cliques, movimentos = [], []
    monkeypatch.setattr(tela.pyautogui, "click", lambda *ponto: cliques.append(ponto))
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda *a, **k: movimentos.append(a))
    return d, estado, cliques, movimentos


@pytest.mark.parametrize("falha", ["foco", "coberta", "movida", "fechada", "fora"])
def test_clique_inseguro_e_recusado(ambiente, falha):
    d, estado, cliques, movimentos = ambiente
    esperada = dict(d.regiao)
    x, y = 50, 60
    if falha == "foco": estado["foco"] = 99
    if falha == "coberta": estado["janela"] = 99
    if falha == "movida": estado["regiao"]["left"] = 200
    if falha == "fechada": estado["aberta"] = False
    if falha == "fora": x = 600
    assert not d.clicar_verificado(x, y, regiao_esperada=esperada)
    assert not cliques and not movimentos


def test_reconfere_foco_depois_de_mover_mouse(ambiente, monkeypatch):
    d, estado, cliques, _ = ambiente
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda *a, **k: estado.update(foco=99))
    assert not d.clicar_verificado(50, 60, regiao_esperada=dict(d.regiao))
    assert not cliques


def test_clique_valido_converte_coordenadas_e_retira_hover(ambiente):
    d, _, cliques, movimentos = ambiente
    assert d.clicar_verificado(50, 60, regiao_esperada=dict(d.regiao))
    assert cliques == [(150, 110)]
    assert movimentos == [(150, 110), (105, 55)]
