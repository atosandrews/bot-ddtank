"""Itens 4 e 5 (29/10): Pronto em uma visita por convidada e convidadas esperando no Cais.

Janelas falsas: cada "frame" é um dict com o nome da tela e os templates visíveis
(nome do arquivo → posição). Nada de mouse, teclado ou janela real.
"""
from types import SimpleNamespace

import pytest

from src import config, grupo
from src.grupo import CANCELE, SAIR_SALA, TROQUE, Grupo


class Janela:
    def __init__(self, nome, telas_seq):
        self.nome = nome
        self.telas = list(telas_seq)  # frames em ordem; o último fica
        self.cliques = []
        self.ao_clicar = None

    def capturar(self):
        return self.telas[0]

    def clicar(self, x, y):
        self.cliques.append((x, y))
        if self.ao_clicar:
            self.ao_clicar(self, (x, y))


def frame(tela, **visiveis):
    return {"tela": tela, **visiveis}


def conta(nome, janela, estagio="FORMIGAS"):
    c = SimpleNamespace(nome=nome, driver=janela, batalha=SimpleNamespace(estagio=estagio),
                        aceitou_nesta_volta=False, pronto_sala=False, clicou_pronto=0,
                        na_luta=False, feitos=[])
    c.log = lambda msg: None
    c.salvar_debug = lambda f, motivo: None
    c.aceitar_convite = lambda f=None: False
    c.fazer_cliques = lambda tela, f: c.feitos.append(tela.nome)
    return c


@pytest.fixture
def ambiente(monkeypatch):
    visitas = []
    monkeypatch.setattr(grupo.vision, "achar",
                        lambda template, f, limiar=None: f.get(template.name) if isinstance(f, dict) else None)
    monkeypatch.setattr(grupo.telas, "identificar",
                        lambda f, limiar, ignorar=(): SimpleNamespace(nome=f["tela"], cliques=["x"], batalha=False)
                        if f.get("tela") else None)
    monkeypatch.setattr(grupo.time, "sleep", lambda s: None)
    monkeypatch.setattr(Grupo, "olhar", staticmethod(lambda c: visitas.append(c.nome) or c.driver.capturar()))
    monkeypatch.setattr(Grupo, "esperar_imagem", staticmethod(lambda c, tpl, lim, seg: c.driver.capturar()))
    monkeypatch.setattr(config, "CONVIDAR_SO_NO_ESTAGIO_2", True)
    return visitas


def montar(convidadas, principal_frame):
    g = Grupo.__new__(Grupo)
    g.principal = conta("principal", Janela("principal", [principal_frame]))
    g.convidadas = convidadas
    g.contas = [g.principal] + convidadas
    g.convidar_direto, g.convidadas_no_cais, g.ultimo_convite = False, True, 10**12
    return g


def sala_com_troque():
    """Janela da convidada na sala; clicar em Pronto troca o botão por 'Cancele'."""
    j = Janela("c", [frame("SALA_PRONTA", **{TROQUE.name: (100, 50)})])

    def pronto(janela, pos):
        if pos == (170, 50):
            janela.telas = [frame("SALA_PRONTA", **{CANCELE.name: (170, 50)})]
    j.ao_clicar = pronto
    return j


# ---------- item 4 ----------

def test_pronto_e_dado_numa_visita_e_depois_nao_volta_nas_convidadas(ambiente, monkeypatch):
    convidadas = [conta(f"conta{i}", sala_com_troque()) for i in (2, 3, 4)]
    for c in convidadas:
        c.aceitou_nesta_volta = True
    g = montar(convidadas, frame("SALA_PRONTA"))
    monkeypatch.setattr(grupo, "contar_prontos", lambda f: 3)

    for c in convidadas:  # passada de Pronto logo após aceitar (em convidar)
        assert g.dar_pronto_na_sala(c) is True
    assert [c.driver.cliques for c in convidadas] == [[(170, 50)]] * 3
    ambiente.clear()

    g.cuidar_da_sala(g.principal)
    assert ambiente == ["principal"]  # nenhuma troca para as convidadas
    assert g.principal.feitos == ["SALA_PRONTA"]  # principal deu Início
    assert all(c.na_luta and not c.pronto_sala for c in convidadas)


def test_pronto_ja_marcado_nao_e_clicado_de_novo(ambiente):
    j = Janela("c", [frame("SALA_PRONTA", **{CANCELE.name: (170, 50), TROQUE.name: (100, 50)})])
    c = conta("conta2", j)
    g = montar([c], frame("SALA_PRONTA"))
    assert g.dar_pronto_na_sala(c) is True
    assert j.cliques == []  # já estava com 'Cancele': clicar tiraria o Pronto


# ---------- item 5 ----------

def test_convidada_em_sala_sai_ate_o_cais(ambiente):
    j = Janela("c", [frame("SALA_PRONTA", **{SAIR_SALA.name: (1400, 770)})])
    j.ao_clicar = lambda janela, pos: setattr(janela, "telas", [frame("CAIS_EXPEDICAO")])
    c = conta("conta2", j)
    g = montar([c], frame("SALA_PRONTA"))
    assert g.levar_ao_cais(c) is True
    assert j.cliques == [(1400, 770)]


def test_no_lobby_nunca_clica_em_sair(ambiente):
    # Mesmo que algo parecido com a porta apareça no lobby, lá Sair fecharia o jogo.
    j = Janela("c", [frame("LOBBY", **{SAIR_SALA.name: (1400, 770)})])
    c = conta("conta2", j)
    c.fazer_cliques = lambda tela, f: setattr(j, "telas", [frame("CAIS_EXPEDICAO")])
    g = montar([c], frame("SALA_PRONTA"))
    assert g.levar_ao_cais(c) is True
    assert j.cliques == []


def test_antes_do_estagio_1_convidadas_saem_da_sala_e_voltam_a_esperar_convite(ambiente, monkeypatch):
    janelas = []
    for _ in range(3):
        j = Janela("c", [frame("SALA_PRONTA", **{SAIR_SALA.name: (1400, 770)})])
        j.ao_clicar = lambda janela, pos: setattr(janela, "telas", [frame("CAIS_EXPEDICAO")])
        janelas.append(j)
    convidadas = [conta(f"conta{i}", j) for i, j in zip((2, 3, 4), janelas)]
    for c in convidadas:
        c.aceitou_nesta_volta, c.pronto_sala = True, True
    g = montar(convidadas, frame("SALA_PRONTA"))
    g.convidadas_no_cais = False
    g.principal.batalha.estagio = "PANDORA"  # acabou a Pandora: próxima é o estágio 1
    monkeypatch.setattr(Grupo, "xis_dos_cartoes", staticmethod(lambda f: [(1, 1)]))
    monkeypatch.setattr(grupo, "contar_prontos", lambda f: 0)

    g.cuidar_da_sala(g.principal)
    assert all(j.cliques == [(1400, 770)] for j in janelas)
    assert all(not c.aceitou_nesta_volta and not c.pronto_sala for c in convidadas)
    assert g.convidadas_no_cais and g.convidar_direto


def test_convidada_em_sala_de_outra_pessoa_sai_em_vez_de_dar_pronto(ambiente):
    j = Janela("c", [frame("SALA_PRONTA", **{TROQUE.name: (100, 50), SAIR_SALA.name: (1400, 770)})])
    j.ao_clicar = lambda janela, pos: setattr(janela, "telas", [frame("CAIS_EXPEDICAO")])
    c = conta("conta2", j)  # não aceitou o convite da principal nesta volta
    g = montar([c], frame("SALA_PRONTA"))
    g.cuidar_da_sala(g.principal)
    assert j.cliques == [(1400, 770)]  # Sair, nunca o Pronto (170, 50)
    assert not c.pronto_sala
