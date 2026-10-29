from pathlib import Path

import cv2
import pytest

from src import bot as modulo_bot, config, telas, verificacao
from src.bot import Bot

RAIZ = Path(__file__).resolve().parents[1]
# Print real da sala logo depois da Pandora: "Selecione a instância" com fundo de personagens.
POS_INSTANCIA = RAIZ / "docs/especificacao/imagens/39_sala_pos_instancia_janela_real.png"


class Driver:
    def __init__(self, frame):
        self.frame = frame
        self.cliques = []

    def capturar(self):
        return self.frame

    def clicar(self, x, y):
        self.cliques.append((x, y))

    def afastar_mouse(self):
        pass


@pytest.fixture
def bot_depois_da_pandora(monkeypatch):
    frame = cv2.imread(str(POS_INSTANCIA))
    assert frame is not None
    monkeypatch.setattr(modulo_bot.time, "sleep", lambda s: None)
    monkeypatch.setattr(config, "SALVAR_PRINTS", False)
    monkeypatch.setattr(verificacao, "tem_codigo", lambda f: False)
    b = Bot(Driver(frame), "teste")
    b.estado, b.batalha.estagio = "BATALHA", config.ULTIMO_ESTAGIO
    return b


def test_sala_depois_da_instancia_e_reconhecida_sem_instancia(bot_depois_da_pandora):
    tela = telas.identificar(bot_depois_da_pandora.driver.frame, config.LIMIAR)
    assert tela.nome == "SALA_SEM_INSTANCIA"


def test_looping_escolhe_a_instancia_de_novo_em_vez_de_clicar_inicio(bot_depois_da_pandora, monkeypatch):
    monkeypatch.setattr(config, "PARAR_DEPOIS_DA_INSTANCIA", False)
    b = bot_depois_da_pandora
    assert b.passo() is True
    assert len(b.driver.cliques) == 1
    x, y = b.driver.cliques[0]
    # Centro do ícone laranja de "Selecione a instância" nesse print (conferido visualmente).
    assert abs(x - 1060) <= 12 and abs(y - 472) <= 12


def test_com_parada_ligada_o_bot_ainda_para_no_fim_da_instancia(bot_depois_da_pandora, monkeypatch):
    monkeypatch.setattr(config, "PARAR_DEPOIS_DA_INSTANCIA", True)
    monkeypatch.setattr(Bot, "salvar_debug", lambda self, frame, motivo: None)
    assert bot_depois_da_pandora.passo() is False
    assert bot_depois_da_pandora.driver.cliques == []
