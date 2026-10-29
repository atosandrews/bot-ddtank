"""Telas que o bot reconhece e o que fazer em cada uma.

Baseado em docs/especificacao/ESPECIFICACAO.md. Para ensinar uma tela nova ao bot,
basta recortar os templates em img/ e adicionar uma entrada em TELAS.
"""
from dataclasses import dataclass, field
from pathlib import Path

from . import vision

IMG = Path(__file__).resolve().parent.parent / "img"


@dataclass
class Clique:
    alvo: Path                  # template do alvo do clique
    deslocamento: tuple = (0, 0)  # ajuste do clique em relação ao centro do alvo
    opcional: bool = False      # se o alvo não aparecer, segue em frente sem ele
    nome: str = ""              # nome no log (padrão: nome do template)


@dataclass
class Tela:
    nome: str
    reconhecer: Path            # template que identifica a tela
    cliques: list = field(default_factory=list)  # feitos em ordem, esperando cada alvo aparecer
    final: bool = False         # depois dos cliques o fluxo conhecido termina
    batalha: bool = False       # em vez de cliques, joga o turno (src/batalha.py)


# A ordem importa: popups vêm antes das telas que ficam por baixo deles.
TELAS = [
    # O jogo perdeu o foco do teclado (ex.: clicaram em outra janela): escurece a tela,
    # ignora as teclas e só volta com um clique dentro dele.
    Tela("SEM_FOCO", IMG / "clique_no_jogo.png", [Clique(IMG / "clique_no_jogo.png")]),
    # O rótulo "Força" da barra de tiro só existe na batalha.
    Tela("BATALHA", IMG / "batalha_forca.png", batalha=True),
    Tela("LEMBRETE", IMG / "lembrete_titulo.png", [Clique(IMG / "botao_ok_lembrete.png")]),
    Tela("ALUNO_RECOMENDADO", IMG / "aluno_recomendado_titulo.png", [Clique(IMG / "botao_fechar_x.png")]),
    # Janela "Configuração da sala". O Troque continua visível atrás dela,
    # então ela precisa vir antes de SALA_PRONTA.
    Tela("SELECAO_INSTANCIA", IMG / "config_sala_titulo.png", [
        Clique(IMG / "formigueiro.png"),
        Clique(IMG / "dificuldade_normal.png"),
        Clique(IMG / "botao_ok_instancia.png"),
    ]),
    Tela("SALA_SEM_INSTANCIA", IMG / "selecione_instancia.png", [Clique(IMG / "selecione_instancia.png")]),
    # O botão Início é animado, então o bot acha o Troque (parado) e clica ao lado dele.
    # Depois do Início pode aparecer o aviso de "lutar sozinho"; se aparecer, confirma.
    Tela("SALA_PRONTA", IMG / "troque.png", [
        Clique(IMG / "troque.png", deslocamento=(70, 0), nome="Início (ao lado do Troque)"),
        Clique(IMG / "botao_ok_lembrete.png", opcional=True),
    ]),
    Tela("CAIS_EXPEDICAO", IMG / "cais_expedicao_titulo.png", [Clique(IMG / "botao_jogar.png")]),
    Tela("SELECAO_CANAL", IMG / "canal_2.png", [Clique(IMG / "canal_2.png")]),
    # O template é o nome "Central"; o deslocamento leva o clique para o prédio logo abaixo.
    Tela("LOBBY", IMG / "central.png", [Clique(IMG / "central.png", deslocamento=(0, 20))]),
]


def identificar(frame, limiar, ignorar=()):
    """Retorna a primeira Tela reconhecida no frame, ou None. `ignorar`: nomes a pular."""
    for tela in TELAS:
        if tela.nome in ignorar:
            continue
        if vision.achar(tela.reconhecer, frame, limiar):
            return tela
    return None
