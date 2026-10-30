from pathlib import Path
from types import SimpleNamespace

import cv2

from src import config
from src.grupo import Grupo

IMAGENS = Path(__file__).resolve().parent / "imagens"


def test_lista_de_amigos_acha_as_tres_contas():
    # Print do usuário (29/10): aba Amigos com Guestf95c4, fakeddt1 e eliseu.
    frame = cv2.imread(str(IMAGENS / "lista_amigos_3_contas.png"))
    achados = dict(Grupo.botoes_das_contas(frame))
    assert set(achados) == {"Guestf95c4", "fakeddt1", "eliseu"}
    # Cada botão Convidar fica na linha do próprio nome.
    assert achados["Guestf95c4"][1] < achados["fakeddt1"][1] < achados["eliseu"][1]


def conta(nome, estagio=None):
    return SimpleNamespace(nome=nome, batalha=SimpleNamespace(estagio=estagio),
                           log=lambda msg: None)


def grupo_falso(monkeypatch, estagio):
    g = Grupo.__new__(Grupo)
    g.principal = conta("principal", estagio)
    g.convidadas = [conta("conta2"), conta("conta3"), conta("conta4")]
    g.contas = [g.principal] + g.convidadas
    g.convidar_direto = False
    g.olhadas, g.convites = [], []
    monkeypatch.setattr(config, "CONVIDAR_SO_NO_ESTAGIO_2", True)
    monkeypatch.setattr(g, "olhar", lambda c: g.olhadas.append(c.nome) or "frame")
    monkeypatch.setattr(g, "convidar", lambda c, f, nomes: g.convites.append((c.nome, nomes)) or True)
    monkeypatch.setattr(g, "sala_sozinha", lambda p: None)
    return g


def test_antes_do_estagio_1_marca_para_convidar_direto_na_volta(monkeypatch):
    g = grupo_falso(monkeypatch, estagio=None)
    g.cuidar_da_sala(g.principal)
    assert g.convidar_direto is True
    assert g.convites == []


def test_depois_do_estagio_1_convida_as_tres_sem_passar_nas_outras_janelas(monkeypatch):
    g = grupo_falso(monkeypatch, estagio="FORMIGAS")
    g.convidar_direto = True
    g.cuidar_da_sala(g.principal)
    assert g.olhadas == ["principal"]  # nenhuma convidada visitada antes do convite
    assert g.convites == [("principal", ["conta2", "conta3", "conta4"])]
    assert g.convidar_direto is False


def test_convite_que_falhou_tenta_de_novo_na_proxima_volta(monkeypatch):
    g = grupo_falso(monkeypatch, estagio="FORMIGAS")
    g.convidar_direto = True
    monkeypatch.setattr(g, "convidar", lambda c, f, nomes: False)
    g.cuidar_da_sala(g.principal)
    assert g.convidar_direto is True
