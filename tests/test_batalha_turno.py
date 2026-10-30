"""Itens pedidos em 29/10: PASS no 1º turno do estágio 1, ângulo antes da vez e fim do 29/30."""
from pathlib import Path

import cv2
import numpy as np
import pytest

from src import batalha, config
from src.batalha import Batalha

ESPEC = Path(__file__).resolve().parents[1] / "docs/especificacao/imagens"
NA_VEZ = ESPEC / "24_formigas_minha_vez_janela_real.png"        # estágio 1, PASS em (960, 403)
FORA_DA_VEZ = ESPEC / "19_batalha_inicio_janela_real.png"       # estágio 1, ainda não é a vez


class Driver:
    def __init__(self, frame=None):
        self.frame = frame
        self.cliques, self.teclas = [], []

    def capturar(self):
        return self.frame

    def clicar(self, x, y):
        self.cliques.append((x, y))

    def tecla(self, tecla, duracao=0.05):
        self.teclas.append(tecla)


@pytest.fixture(autouse=True)
def sem_espera(monkeypatch):
    monkeypatch.setattr(batalha.time, "sleep", lambda s: None)


def nova(frame=None):
    linhas = []
    b = Batalha(Driver(frame), linhas.append, lambda *a: None, config.ESTRATEGIAS)
    b.linhas = linhas
    return b


def test_primeiro_turno_do_estagio_1_so_passa_e_o_segundo_joga(monkeypatch):
    frame = cv2.imread(str(NA_VEZ))
    b = nova(frame)
    jogadas = []
    monkeypatch.setattr(b, "jogar", lambda *a, **k: jogadas.append(a))
    monkeypatch.setattr(b, "esperar_fim_da_vez", lambda: None)

    b.passo(frame)
    assert b.driver.cliques == [(960, 403)] and jogadas == []
    b.passo(frame)
    assert len(jogadas) == 1  # da 2ª vez em diante: ângulo 30, força 10 como antes
    estrategia = jogadas[0][1]
    assert (estrategia["angulo"], estrategia["forca"]) == (30, 10)


def test_nova_luta_passa_de_novo_no_estagio_1(monkeypatch):
    frame = cv2.imread(str(NA_VEZ))
    b = nova(frame)
    monkeypatch.setattr(b, "jogar", lambda *a, **k: None)
    monkeypatch.setattr(b, "esperar_fim_da_vez", lambda: None)
    b.passo(frame)
    b.nova_luta()
    b.passo(frame)
    assert len(b.driver.cliques) == 2


def test_pandora_nao_passa_o_primeiro_turno(monkeypatch):
    frame = cv2.imread(str(ESPEC / "26_pandora_minha_vez_janela_real.png"))
    b = nova(frame)
    jogadas = []
    monkeypatch.setattr(b, "jogar", lambda *a, **k: jogadas.append(a))
    b.passo(frame)
    assert len(jogadas) == 1 and b.driver.cliques == []


def test_fora_da_vez_ajusta_o_angulo_uma_vez_por_estagio(monkeypatch):
    frame = cv2.imread(str(FORA_DA_VEZ))
    b = nova(frame)
    pedidos = []
    monkeypatch.setattr(b, "ajustar_angulo", lambda ancora, alvo: pedidos.append(alvo) or True)
    b.passo(frame)
    b.passo(frame)
    assert pedidos == [30]
    b.nova_luta()
    b.passo(frame)
    assert pedidos == [30, 30]


class Relogio:
    """Relógio de ângulo falso: cada toque anda `passo` graus (2 = o caso que oscila)."""

    def __init__(self, angulo, passo):
        self.angulo, self.passo = angulo, passo

    def tecla(self, tecla, duracao=0.05):
        self.angulo += self.passo if tecla == config.TECLA_ANGULO_SOBE else -self.passo


def com_relogio(monkeypatch, relogio):
    b = nova()
    b.driver.tecla = relogio.tecla
    b.driver.capturar = lambda: relogio.angulo
    monkeypatch.setattr(batalha, "ler_numero", lambda frame, ancora: (frame, True))
    # Máscara do número: vários pixels, como a de verdade (o bot só vê "mudou" com > 3).
    monkeypatch.setattr(batalha, "mascara_numero", lambda frame, ancora: np.full(20, frame))
    return b


def test_angulo_oscilando_29_31_para_e_atira(monkeypatch):
    relogio = Relogio(29, 2)  # 29 → 31 → 29...: nunca mostra 30
    b = com_relogio(monkeypatch, relogio)
    assert b.ajustar_angulo((0, 0), 30) is True
    assert any("oscilando" in l for l in b.linhas)


def test_angulo_normal_chega_exato(monkeypatch):
    relogio = Relogio(26, 1)
    b = com_relogio(monkeypatch, relogio)
    assert b.ajustar_angulo((0, 0), 30) is True
    assert relogio.angulo == 30


def test_ajuste_tem_limite_de_tempo(monkeypatch):
    relogio = Relogio(29, 2)
    b = com_relogio(monkeypatch, relogio)
    agora = iter(range(0, 1000, 3))  # cada leitura do relógio do sistema avança 3 s
    monkeypatch.setattr(batalha.time, "time", lambda: next(agora))
    assert b.ajustar_angulo((0, 0), 30) is True  # 29 está a 1°: atira
    assert any("tempo do ajuste esgotado" in l for l in b.linhas)
