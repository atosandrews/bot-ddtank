"""Grupo do Pintinho sem o jogo: banner da sala, decisão de convidar e a troca temporária
do que src/grupo.py usa do Formigueiro. Prints em tests/imagens/pintinho/ (gravação de 30/10)."""
from pathlib import Path

import cv2
import numpy as np
import pytest

from src import batalha, config, grupo, telas
from src import grupo_pintinho as gp
from src import pintinho as pt

PASTA = Path(__file__).resolve().parent / "imagens" / "pintinho"


def img(nome):
    frame = cv2.imread(str(PASTA / f"{nome}.png"))
    assert frame is not None, nome
    return frame


@pytest.mark.parametrize("nome, estagio", [
    ("sala_estagio_1", None), ("sala_stage_3", 3), ("sala_stage_4", 4), ("e2_0de3_vento-1.2", None),
])
def test_banner_da_sala_diz_o_proximo_estagio(nome, estagio):
    assert gp.proximo_estagio(img(nome)) == estagio


def test_modo_pintinho_troca_e_devolve_o_que_o_grupo_usa():
    antes = (grupo.telas, grupo.ler_batalha, config.SO_PRINCIPAL_JOGA,
             config.CONVIDAR_SO_NO_ESTAGIO_2, config.CONFIAR_CONVIDADAS)
    with gp.modo_pintinho():
        assert grupo.telas is not telas and grupo.ler_batalha is gp.ler_sem_formigas
        assert config.CONFIAR_CONVIDADAS and config.SO_PRINCIPAL_JOGA
        selecao = next(t for t in grupo.telas.TELAS if t.nome == "SELECAO_INSTANCIA")
        assert [c.alvo for c in selecao.cliques] == pt.SELECAO  # Pintinho → Difícil → ok
    assert (grupo.telas, grupo.ler_batalha, config.SO_PRINCIPAL_JOGA,
            config.CONVIDAR_SO_NO_ESTAGIO_2, config.CONFIAR_CONVIDADAS) == antes


def test_convidada_no_pintinho_so_liga_o_confiar():
    # No grupo do Formigueiro, estágio "FORMIGAS" faz a convidada andar até a principal.
    leitura = gp.ler_sem_formigas(img("e4"))
    assert leitura is not None and leitura.estagio == "PINTINHO"
    assert gp.ler_sem_formigas(img("sala_stage_4")) is None


class Conta:
    def __init__(self, frame, estagio):
        self.driver = type("D", (), {"capturar": lambda s, sub=None: frame})()
        self.batalha = type("B", (), {"estagio": estagio})()


def grupo_de_teste(monkeypatch, convidadas=()):
    g = gp.GrupoPintinho.__new__(gp.GrupoPintinho)  # sem janelas do jogo
    g.convidar_direto = False
    g.convidadas = list(convidadas)
    chamadas = []
    monkeypatch.setattr(g, "sala_sozinha", lambda principal: chamadas.append("sozinha"))
    monkeypatch.setattr(grupo.Grupo, "cuidar_da_sala",
                        lambda self, principal: chamadas.append(("grupo", config.CONVIDAR_SO_NO_ESTAGIO_2)))
    return g, chamadas


@pytest.mark.parametrize("sala, ultimo_jogado, esperado", [
    ("sala_stage_4", 3, [("grupo", False)]),   # banner STAGE 4: convida (caminho do grupo)
    ("sala_stage_3", 2, ["sozinha"]),          # antes do 4: vai sozinha
    ("sala_estagio_1", 4, ["sozinha"]),        # fim da instância: recomeça sozinha (expulsa)
])
def test_so_convida_antes_do_estagio_4(monkeypatch, sala, ultimo_jogado, esperado):
    g, chamadas = grupo_de_teste(monkeypatch)
    with gp.modo_pintinho():
        g.cuidar_da_sala(Conta(img(sala), ultimo_jogado))
        assert config.CONVIDAR_SO_NO_ESTAGIO_2  # volta ao normal depois do caminho do grupo
    assert chamadas == esperado
    assert g.convidar_direto == (esperado == ["sozinha"])


def test_print_invalido_nao_derruba_o_grupo(monkeypatch):
    # 1º teste do grupo (30/10): cv2.error com a janela da principal sem imagem derrubou o bot.
    g = gp.GrupoPintinho.__new__(gp.GrupoPintinho)
    monkeypatch.setattr(g, "virar_cartas", lambda: False)
    monkeypatch.setattr(grupo.Grupo, "passo", lambda self: cv2.cvtColor(np.zeros((0, 0, 3), np.uint8), cv2.COLOR_BGR2HSV))
    monkeypatch.setattr(pt.time, "sleep", lambda s: None)
    soltas = []
    monkeypatch.setattr(pt, "soltar_teclas", lambda: soltas.append(True))
    g.passo()  # não levanta
    assert soltas == [True]
    assert grupo.telas is telas  # a troca do Pintinho foi desfeita mesmo com o erro


def test_convidada_ja_na_sala_do_estagio_4_conta_como_aceita(monkeypatch):
    dentro, fora = Conta(img("sala_stage_4"), None), Conta(img("e1"), None)
    for c in (dentro, fora):
        c.aceitou_nesta_volta, c.log = False, lambda m: None
    g, chamadas = grupo_de_teste(monkeypatch, [dentro, fora])
    with gp.modo_pintinho():
        g.cuidar_da_sala(Conta(img("sala_stage_4"), 3))
    assert dentro.aceitou_nesta_volta and not fora.aceitou_nesta_volta
