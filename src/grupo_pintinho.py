"""Pintinho Corre Rápido em grupo: a principal faz os estágios 1 a 3 sozinha e chama as outras
contas só para o estágio 4 (Burrinho e gaiola). No fim, expulsa e recomeça.

Uso: python -m src.grupo_pintinho                    (ou rodar_grupo_pintinho.bat)
     python -m src.grupo_pintinho PRINCIPAL C1 C2 C3  (números das janelas)
Sem números, pede um clique na janela da principal e usa as outras janelas do jogo abertas.

Reaproveita o grupo do Formigueiro (src/grupo.py: convite pela aba Amigos, Pronto, expulsar
pelo X, captcha) sem mudá-lo: durante cada passo, as telas, a leitura da batalha e as opções
do grupo são trocadas pelas do Pintinho (`modo_pintinho`) e voltam ao normal depois.

- Sala: o banner "STAGE N" diz o próximo estágio. Antes do 4, a principal expulsa quem estiver
  na sala e inicia sozinha; antes do 4, convida as outras pela aba Amigos e espera o Pronto.
- Batalha: a principal joga como em src/pintinho.py. No estágio 4, depois do 1º tiro dela, o
  bot passa em cada convidada, liga o Confiar (o jogo joga por ela) e passa a vez (usuário, 30/10).
- Cartas: cada conta vira as grátis (1 por estágio, 2 no fim).
"""
import contextlib
import random
import sys
import time
from types import SimpleNamespace

from . import config, grupo, vision
from . import pintinho as pt
from . import pintinho_visao as pv
from .batalha import ler as ler_batalha
from .grupo import Grupo, avisar
from .telas import Clique, Tela

# As telas do Pintinho com a escolha da instância em cliques (o grupo usa fazer_cliques).
TELAS = [Tela(t.nome, t.reconhecer, [Clique(alvo) for alvo in pt.SELECAO])
         if t.nome == "SELECAO_INSTANCIA" else t for t in pt.TELAS]

# Banner do quadro da sala ("STAGE 2", "STAGE 3", "STAGE 4"): o número grande. Antes do 1º
# estágio aparece o nome da instância. Recortes da gravação de 30/10 (cliente 1920x1032).
SALA_STAGE = {n: pv.PASTA / f"sala_stage_{n}.png" for n in (2, 3, 4)}
LIMIAR_SALA_STAGE = 0.8
ULTIMO_ESTAGIO = 4


def identificar(frame, limiar, ignorar=()):
    for tela in TELAS:
        if tela.nome not in ignorar and vision.achar(tela.reconhecer, frame, limiar):
            return tela
    return None


def ler_sem_formigas(frame):
    """Leitura da batalha para src/grupo.py: lá o estágio "FORMIGAS" faz a convidada andar até
    a principal antes do Confiar; no Pintinho ela só liga o Confiar e passa."""
    leitura = ler_batalha(frame)
    if leitura is not None:
        leitura.estagio = "PINTINHO"
    return leitura


def proximo_estagio(frame):
    """Estágio que a sala vai começar (2, 3 ou 4) pelo banner, ou None (estágio 1 / ilegível)."""
    notas = {}
    for n, tpl in SALA_STAGE.items():
        _, nota = vision.procurar(tpl, frame)
        notas[n] = nota
    n = max(notas, key=notas.get)
    return n if notas[n] >= LIMIAR_SALA_STAGE else None


@contextlib.contextmanager
def modo_pintinho():
    """Troca, só enquanto dura, o que src/grupo.py usa do Formigueiro pelo do Pintinho."""
    antes = (grupo.telas, grupo.ler_batalha, config.SO_PRINCIPAL_JOGA,
             config.CONVIDAR_SO_NO_ESTAGIO_2, config.CONFIAR_CONVIDADAS)
    grupo.telas = SimpleNamespace(identificar=identificar, TELAS=TELAS)
    grupo.ler_batalha = ler_sem_formigas
    config.SO_PRINCIPAL_JOGA = True       # na batalha só a principal joga
    config.CONVIDAR_SO_NO_ESTAGIO_2 = True  # "sozinha antes do último": decidido em cuidar_da_sala
    config.CONFIAR_CONVIDADAS = True      # estágio 4: convidadas no Confiar depois da principal
    try:
        yield
    finally:
        (grupo.telas, grupo.ler_batalha, config.SO_PRINCIPAL_JOGA,
         config.CONVIDAR_SO_NO_ESTAGIO_2, config.CONFIAR_CONVIDADAS) = antes


def esperar_clique_real(hwnds, segundos=60):
    """A janela do jogo que levar um clique NOVO do botão esquerdo depois de iniciar.

    O grupo do Formigueiro pega a janela que está na frente: se uma janela do jogo já estava na
    frente ao iniciar, ela vira a principal sem clique nenhum (30/10: pegou uma secundária).
    """
    import win32api
    import win32gui
    print("Clique na janela da CONTA PRINCIPAL (a dona da sala)...", flush=True)
    apertado = bool(win32api.GetAsyncKeyState(0x01) & 0x8000)
    limite = time.time() + segundos
    while time.time() < limite:
        agora = bool(win32api.GetAsyncKeyState(0x01) & 0x8000)
        if agora and not apertado:
            janela = win32gui.GetAncestor(win32gui.WindowFromPoint(win32api.GetCursorPos()), 2)
            if janela in hwnds:
                return janela
        apertado = agora
        time.sleep(0.02)
    return None


class GrupoPintinho(Grupo):
    def __init__(self, principal=None, convidadas=None):
        super().__init__(principal, convidadas)
        p = self.principal
        p.batalha = pt.BatalhaPintinho(p.driver, p.log, p.salvar_debug)
        self.cartas = {c.nome: 0 for c in self.contas}  # cartas viradas na tela atual

    def passo(self):
        with modo_pintinho(), pt.tolerar_print_invalido(avisar):
            if self.virar_cartas():
                return
            self.marcar_convidadas_na_luta()
            super().passo()

    def marcar_convidadas_na_luta(self):
        """Estágio 4: convidada com a batalha na janela está na luta (Confiar e passar), mesmo
        que o Início não tenha passado pelo caminho do grupo que marca isso (11º teste, 30/10)."""
        if not self.na_batalha or self.principal.batalha.estagio != ULTIMO_ESTAGIO:
            return
        for c in self.convidadas:
            if not c.na_luta and not c.confiou and ler_batalha(c.driver.capturar()) is not None:
                c.log("na luta do estágio 4")
                c.na_luta = True

    def levar_ao_cais(self, c, tentativas=6):
        """Antes de tirar uma convidada da "sala de outra pessoa", olha o banner com a janela
        dela na frente: com STAGE 4 é a sala da principal (11º teste: a conta2 entrou atrasada)."""
        if proximo_estagio(self.olhar(c)) == ULTIMO_ESTAGIO:
            c.log("já está na sala do estágio 4 da principal")
            c.aceitou_nesta_volta = True
            return False
        return super().levar_ao_cais(c, tentativas)

    def virar_cartas(self):
        """Tela de cartas em alguma conta (fim de estágio): vira as grátis de cada uma.
        Fora da batalha só; True se alguma conta estava nas cartas."""
        if self.na_batalha:
            return False
        alguma = False
        for conta in self.contas:
            frame = conta.driver.capturar()
            tipo = pv.tela_de_cartas(frame)
            if not tipo:
                self.cartas[conta.nome] = 0
                continue
            alguma = True
            if self.cartas[conta.nome] >= pt.CARTAS_POR_TELA[tipo]:
                continue
            fechadas = pv.cartas_fechadas(frame)
            if not fechadas:
                continue
            x, y = random.choice(fechadas)
            conta.log(f"virando uma carta ({self.cartas[conta.nome] + 1} de {pt.CARTAS_POR_TELA[tipo]})")
            conta.driver.clicar(x, y)
            time.sleep(0.8)
            conta.driver.afastar_mouse()
            if len(pv.cartas_fechadas(conta.driver.capturar())) < len(fechadas):
                self.cartas[conta.nome] += 1
        return alguma

    def proximo_e_o_ultimo(self, principal):
        """O próximo estágio é o 4? Pelo banner da sala; sem ele, pelo último estágio jogado."""
        frame = principal.driver.capturar()
        n = proximo_estagio(frame)
        if n is None and grupo.telas.identificar(frame, config.LIMIAR, ("SEM_FOCO",)) is not None:
            estagio = principal.batalha.estagio
            n = estagio + 1 if isinstance(estagio, int) and estagio < ULTIMO_ESTAGIO else 1
        return n == ULTIMO_ESTAGIO

    def cuidar_da_sala(self, principal):
        if not self.proximo_e_o_ultimo(principal):
            # Estágios 1 a 3: a principal vai sozinha (expulsa quem estiver na sala).
            self.sala_sozinha(principal)
            self.convidar_direto = True
            return
        # Estágio 4: o caminho do grupo (convidar, Pronto e Início com todas prontas).
        # Convidada numa sala com o banner STAGE 4 já está na sala da principal, mesmo que o bot
        # não a tenha visto aceitar: o grupo do Formigueiro a mandaria sair ("sala de outra
        # pessoa"), como a conta4 no 2º teste do grupo (30/10).
        for c in self.convidadas:
            if not c.aceitou_nesta_volta and proximo_estagio(c.driver.capturar()) == ULTIMO_ESTAGIO:
                c.log("já está na sala do estágio 4 da principal")
                c.aceitou_nesta_volta = True
        config.CONVIDAR_SO_NO_ESTAGIO_2 = False
        try:
            super().cuidar_da_sala(principal)
        finally:
            config.CONVIDAR_SO_NO_ESTAGIO_2 = True

    def rodar(self):
        avisar("Pintinho em grupo: estágios 1–3 sozinha, as outras entram no 4")
        return super().rodar()


def main():
    # python -m src.grupo_pintinho [principal] [convidada ...]   (números das janelas)
    from .drivers.conta import contas_abertas
    from .drivers.tela import ativar_dpi
    ativar_dpi()
    numeros = [int(a) for a in sys.argv[1:]]
    principal = numeros[0] if numeros else esperar_clique_real(contas_abertas(), segundos=180)
    if principal is None:
        print("Nenhuma janela do jogo clicada.")
        return 1
    print(f"principal: janela {principal}", flush=True)
    return 0 if GrupoPintinho(principal, numeros[1:]).rodar() else 1


if __name__ == "__main__":
    sys.exit(main())
