"""Fase 3: várias contas juntas no Formigueiro, em ciclo sem parar.

Uso: python -m src.grupo   (ou rodar_grupo.bat)
     python -m src.grupo PRINCIPAL CONVIDADA1 CONVIDADA2   (números das janelas)
Sem números, pede um clique na janela da principal e usa todas as outras janelas do jogo
abertas como convidadas: feche as que não vão jogar.

- A conta principal é a janela maior do jogo; as outras são convidadas.
- Principal na sala e alguma convidada fora dela (Cais, lobby...): convida pela aba Amigos.
- Convidada recebe o convite: Confirmar. Na sala: Pronto (uma vez por volta à sala).
- Principal: escolhe Formigueiro/Normal e só clica em Início com todas as convidadas prontas.
- Batalha: a conta que estiver com a vez joga (config.ESTRATEGIAS_GRUPO).
Janelas sem foco mostram "Clique no jogo" escurecido; para VER a tela o bot ignora o aviso
(a comparação de imagens não se importa com o escurecimento) e só clica nele para AGIR.
"""
import random
import time
from datetime import datetime
from pathlib import Path

import cv2
import numpy as np
import pyautogui

from . import config, telas, verificacao, vision
from .registro import gravar_log
from .batalha import LIMIAR_PASS, PASS as PASS_BATALHA, Batalha, ler as ler_batalha
from .drivers.conta import DriverConta, contas_abertas, esperar_clique_na_principal
from .drivers.tela import ativar_dpi, trazer_para_frente
from .telas import IMG

LOGS = Path(__file__).resolve().parent.parent / "logs"


def avisar(msg):
    print(msg, flush=True)
    gravar_log(f"[{datetime.now():%H:%M:%S}] {msg}")

CONVIDE = IMG / "grupo_convide.png"
ABA_AMIGOS = IMG / "grupo_aba_amigos.png"
CONVIDAR = IMG / "grupo_botao_convidar.png"
CONVITE = IMG / "grupo_convite_titulo.png"
CONFIRMAR = IMG / "grupo_botao_confirmar.png"
CARIMBO = IMG / "grupo_pronto_carimbo.png"
TROQUE = IMG / "troque.png"
CONFIAR = IMG / "batalha_confiar.png"
CANCELE = IMG / "grupo_cancele.png"  # botão da convidada depois de dar Pronto
XIS = IMG / "sala_expulsar_x.png"  # X pequeno do cartão do jogador (a dona expulsa)
PASTA_CONVIDADOS = IMG / "convidados"
TELA_BATALHA = next(t for t in telas.TELAS if t.nome == "BATALHA")  # um recorte do nome de cada conta na lista de amigos

FORA_DA_SALA = {"CAIS_EXPEDICAO", "LOBBY", "SELECAO_CANAL", "ALUNO_RECOMENDADO"}
NA_SALA = {"SALA_SEM_INSTANCIA", "SALA_PRONTA", "SELECAO_INSTANCIA"}


# Cartões dos jogadores na sala, em relação ao centro do Troque (x1, y1, x2, y2).
# Só procuro carimbos ali: a imagem do Formigueiro tem formigas vermelhas que confundiam.
CARTOES = (-860, -470, -470, -60)
# Botões do convite e do Lembrete, em relação ao centro do título / do OK.
CONFIRMAR_DO_TITULO = (99, 180)
CANCELAR_DO_OK = (275, 0)


def contar_prontos(frame):
    """Quantos carimbos 'Pronto!!' aparecem nos cartões da sala (comparando só o vermelho)."""
    troque = vision.achar(TROQUE, frame, config.LIMIAR)
    if troque is None:
        return 0
    x1, y1, x2, y2 = CARTOES
    frame = frame[max(troque[1] + y1, 0):troque[1] + y2, max(troque[0] + x1, 0):troque[0] + x2]
    # Normaliza o brilho: a janela sem foco fica escurecida e o vermelho do carimbo também.
    frame = frame.astype(float) * (255.0 / max(float(np.percentile(frame, 99)), 1.0))
    b, g, r = (np.clip(frame[:, :, k], 0, 255) for k in range(3))
    vermelho = ((r > 150) & (r - g > 70) & (r - b > 60)).astype(np.uint8) * 255
    tpl = cv2.imread(str(CARIMBO), cv2.IMREAD_GRAYSCALE)
    res = cv2.matchTemplate(vermelho, tpl, cv2.TM_CCOEFF_NORMED)
    achados = []
    for y, x in sorted(zip(*np.where(res >= 0.7)), key=lambda p: -res[p]):
        if all(abs(x - a) > 40 or abs(y - c) > 40 for a, c in achados):
            achados.append((x, y))
    return len(achados)


class Conta:
    def __init__(self, hwnd, nome, principal):
        self.nome = nome
        self.principal = principal
        self.driver = DriverConta(hwnd, nome)
        if config.CONVIDAR_SO_NO_ESTAGIO_2:
            estrategias = config.ESTRATEGIAS  # o padrão de uma conta (estágio 1 sozinha)
        elif config.SO_PRINCIPAL_JOGA:
            estrategias = config.ESTRATEGIAS_SO_PRINCIPAL
        else:
            estrategias = config.ESTRATEGIAS_GRUPO
        self.batalha = Batalha(self.driver, self.log, self.salvar_debug, estrategias)
        self.tela = None
        self.pronto = False
        self.aceitou_em = 0.0  # quando aceitou o último convite
        self.confiou = False  # Confiar ligado neste estágio (modo só a principal joga)
        self.clicou_pronto = 0  # voltas desde o último clique em Pronto (0 = não clicou)
        self.na_luta = False  # entrou na luta atual com a principal (estágio 2)

    def log(self, msg):
        linha = f"[{datetime.now():%H:%M:%S}] [{self.nome}] [{self.tela}] {msg}"
        print(linha, flush=True)
        gravar_log(linha)

    def salvar_debug(self, frame, motivo):
        LOGS.mkdir(exist_ok=True)
        arq = LOGS / f"{self.nome}_{datetime.now():%H%M%S}_{motivo}.png"
        cv2.imwrite(str(arq), frame)
        self.log(f"print salvo em {arq.name}")

    def ver(self):
        frame = self.driver.capturar()
        tela = telas.identificar(frame, config.LIMIAR, ignorar=("SEM_FOCO",))
        nome = tela.nome if tela else None
        if nome != self.tela:
            self.tela = nome
            if nome:
                self.log("tela reconhecida")
        # Só zera o Pronto ao entrar na batalha (na volta à sala precisa dar de novo). Janela
        # coberta congela e pode mostrar a tela antiga; zerar por isso faria clicar de novo
        # e tirar o Pronto.
        if tela is not None and tela.batalha:
            self.pronto = False
        return frame, tela

    def aceitar_convite(self, frame=None):
        """Se houver convite na tela, clica em Confirmar. True se aceitou."""
        frame = frame if frame is not None else self.driver.capturar()
        titulo = vision.achar(CONVITE, frame, config.LIMIAR)
        if titulo is None:
            return False
        self.log("convite recebido: confirmando")
        self.aceitou_em = time.time()
        # Direto no Confirmar: o aviso "Clique no jogo" fica no meio e não cobre o botão,
        # e o convite expira em 12 s (não dá tempo de clicar no aviso antes).
        trazer_para_frente(self.driver.hwnd)
        self.driver._atualizar_regiao()
        self.driver._clicar_direto(titulo[0] + CONFIRMAR_DO_TITULO[0], titulo[1] + CONFIRMAR_DO_TITULO[1])
        time.sleep(0.5)
        if vision.achar(CONVITE, self.driver.capturar(), config.LIMIAR):
            self.log("o convite continua na tela; salvando print")
            self.salvar_debug(self.driver.capturar(), "convite")
        return True

    def clicar_em(self, template, frame=None, limiar=None, espera=5):
        """Espera o template aparecer (até `espera` s) e clica nele. True se clicou."""
        limite = time.time() + espera
        while True:
            frame = frame if frame is not None else self.driver.capturar()
            pos = vision.achar(template, frame, limiar or config.LIMIAR)
            if pos:
                self.driver.clicar(*pos)
                time.sleep(random.uniform(*config.ESPERA_CLIQUE))
                return True
            if time.time() >= limite:
                return False
            time.sleep(0.3)
            frame = None

    def fazer_cliques(self, tela, frame):
        """Faz a sequência de cliques de uma tela do fluxo normal (src/telas.py)."""
        for clique in tela.cliques:
            limite = time.time() + config.ESPERA_ALVO
            while True:
                pos = vision.achar(clique.alvo, frame, config.LIMIAR)
                if pos or time.time() >= limite:
                    break
                time.sleep(0.3)
                frame = self.driver.capturar()
            if pos is None:
                if not clique.opcional:
                    self.log(f"{clique.alvo.name} não apareceu")
                    return
                continue
            x, y = pos[0] + clique.deslocamento[0], pos[1] + clique.deslocamento[1]
            self.log(f"clicando em {clique.nome or clique.alvo.stem}")
            self.driver.clicar(x, y)
            time.sleep(random.uniform(*config.ESPERA_CLIQUE))
            frame = self.driver.capturar()


class Grupo:
    def __init__(self, principal=None, convidadas=None):
        ativar_dpi()
        hwnds = contas_abertas()
        if not hwnds:
            raise RuntimeError(f"Nenhuma janela '{config.TITULO_JANELA}' aberta.")
        # A principal: número da janela passado na linha de comando, ou a que o usuário clicar.
        principal = principal or esperar_clique_na_principal(hwnds)
        if principal not in hwnds:
            raise RuntimeError("Não sei qual é a conta principal (clique na janela dela ao iniciar).")
        # Convidadas: as janelas passadas na linha de comando, ou todas as outras abertas.
        if convidadas:
            faltando = [h for h in convidadas if h not in hwnds]
            if faltando:
                raise RuntimeError(f"Janelas não encontradas: {faltando}")
            outras = list(convidadas)
        else:
            outras = [h for h in hwnds if h != principal]
        hwnds = [principal] + outras
        self.contas = [Conta(h, "principal" if i == 0 else f"conta{i + 1}", i == 0)
                       for i, h in enumerate(hwnds)]
        self.principal = self.contas[0]
        self.convidadas = self.contas[1:]
        self.ultimo_convite = 0.0
        self.visita = -1  # rodízio de contas visitadas na batalha
        self.ordem = []  # ordem das vezes na batalha, aprendida nos primeiros turnos
        self.proxima = None  # conta que deve jogar em seguida
        self.principal_jogou = False  # a principal já deu o 1º tiro nesta luta
        self.na_batalha = False  # a principal está na luta (caminho rápido)
        self.ultimo_codigo = 0.0  # última conferência do código de verificação
        avisar(f"{len(self.contas)} contas: principal + {len(self.convidadas)} convidada(s)")

    def passo(self):
        # Verificação em qualquer conta suspende o grupo e usa o controlador serial.
        if time.time() - self.ultimo_codigo > 5:
            self.ultimo_codigo = time.time()
            for conta in self.contas:
                if verificacao.tem_codigo(conta.driver.capturar()):
                    verificacao.tratar(conta.driver, conta.nome)
                    return

        # Caminho rápido na batalha: só a janela da principal (print rápido + vez dela).
        # Reconhecer as telas das 3 janelas leva ~2 s e fazia a principal perder a vez.
        if config.SO_PRINCIPAL_JOGA and self.na_batalha:
            principal = self.principal
            if not principal.driver.na_frente():
                principal.driver.ativar_com_clique()
            frame = principal.driver.capturar()
            if verificacao.tem_codigo(frame):
                verificacao.tratar(principal.driver, principal.nome)
                return
            if ler_batalha(frame) is not None:
                vistas = [(principal, frame, TELA_BATALHA)] + [(c, None, None) for c in self.convidadas]
                if self.batalha_so_principal(vistas):
                    return
            self.na_batalha = False  # saiu da batalha: volta a olhar todas as telas

        # 0) Convite primeiro: ele some em 12 s e reconhecer as telas é lento (~0,7 s por conta).
        for conta in self.convidadas:
            if conta.aceitar_convite():
                return

        vistas = [(c, *c.ver()) for c in self.contas]

        if config.SO_PRINCIPAL_JOGA:
            if self.batalha_so_principal(vistas):
                return
            em_batalha = []
        else:
            em_batalha = None

        # 1) Batalha: quem estiver com a vez joga (uma conta por vez).
        if em_batalha is None:
            em_batalha = [(c, f) for c, f, t in vistas if t and t.batalha]
        if not em_batalha:
            self.ordem, self.proxima = [], None
        for conta, frame in em_batalha:
            leitura = ler_batalha(frame)
            if leitura and leitura.minha_vez:
                self.jogar_vez(conta, frame)
                return
        if em_batalha and self.proxima is not None:
            # A vez segue sempre a mesma ordem: espera a próxima da fila com a janela na frente.
            if self.esperar_vez(self.proxima):
                return
            self.proxima = None
        if em_batalha:
            # Ninguém com a vez à vista: janela coberta fica congelada e não mostra o PASS.
            # Visita a próxima conta da batalha (rodízio) e olha de novo com ela na frente.
            self.visita = (self.visita + 1) % len(em_batalha)
            conta, _ = em_batalha[self.visita]
            trazer_para_frente(conta.driver.hwnd)
            time.sleep(0.8)
            frame = conta.driver.capturar()
            leitura = ler_batalha(frame)
            if leitura and leitura.minha_vez:
                self.jogar_vez(conta, frame)
            return

        # 2) Principal na sala: passa em cada convidada e depois na principal.
        conta, frame, tela = vistas[0]
        if tela is None:
            return
        if tela.nome in NA_SALA:
            self.cuidar_da_sala(conta)
            return
        if tela.batalha:
            return  # batalha, mas não é a vez de ninguém aqui
        if tela.nome == "LEMBRETE":
            # "Deseja combater sozinho?": em grupo, nunca. Cancela e espera as convidadas.
            ok = vision.achar(IMG / "botao_ok_lembrete.png", frame, config.LIMIAR)
            if ok:
                conta.log("aviso de lutar sozinho: cancelando")
                conta.driver.clicar(ok[0] + CANCELAR_DO_OK[0], ok[1] + CANCELAR_DO_OK[1])
            return
        if tela.cliques:
            conta.fazer_cliques(tela, frame)
            time.sleep(random.uniform(*config.ESPERA_TRANSICAO))

    def cuidar_da_sala(self, principal):
        """Uma volta pela sala: troca para cada convidada (com clique) e vê a tela atual.

        - convite na tela: Confirmar;
        - botão 'Cancele': já está pronta;
        - botão 'Pronto' (no lugar do Início, ao lado do Troque): clica e confere o 'Cancele';
        - fora da sala: precisa de convite.
        Com todas prontas, vai para a principal (escolhe a instância se precisar) e dá Início.
        """
        if config.CONVIDAR_SO_NO_ESTAGIO_2 and principal.batalha.estagio != "FORMIGAS":
            # Próxima luta é o estágio 1: a principal vai sozinha. Convidada na sala sai.
            self.sala_sozinha(principal)
            return

        fora, prontas = [], 0
        for c in self.convidadas:
            frame = self.olhar(c)
            if c.aceitar_convite(frame):
                # Espera a sala carregar (até 3 s), seguindo assim que o Troque aparecer.
                frame = self.esperar_imagem(c, TROQUE, config.LIMIAR, 3.0)
            if vision.achar(CANCELE, frame, 0.75):
                prontas += 1
                continue
            troque = vision.achar(TROQUE, frame, config.LIMIAR)
            if troque is None:
                fora.append(c)
                c.clicou_pronto = 0
                continue
            # O mesmo botão marca e desmarca o Pronto: só clica de novo depois de 3 voltas
            # sem ver o 'Cancele' (o clique pode não ter pegado).
            if c.clicou_pronto and c.clicou_pronto < 3:
                c.clicou_pronto += 1
                continue
            c.log("clicando em Pronto")
            c.driver.clicar(troque[0] + 70, troque[1])
            c.clicou_pronto = 1
            # Segue assim que o botão virar 'Cancele' (até 1,5 s).
            if vision.achar(CANCELE, self.esperar_imagem(c, CANCELE, 0.75, 1.5), 0.75):
                prontas += 1
            else:
                c.log("o botão não virou 'Cancele' ainda")
                c.salvar_debug(c.driver.capturar(), "sem_cancele")

        frame = self.olhar(principal)
        tela = telas.identificar(frame, config.LIMIAR, ignorar=("SEM_FOCO",))
        if tela is None:
            return
        # Os carimbos na sala da principal também contam (o 'Cancele' pode não ser visto).
        if tela.nome == "SALA_PRONTA":
            prontas = max(prontas, contar_prontos(frame))
        if tela.nome == "LEMBRETE":
            ok = vision.achar(IMG / "botao_ok_lembrete.png", frame, config.LIMIAR)
            if ok:
                principal.log("aviso de lutar sozinho: cancelando")
                principal.driver.clicar(ok[0] + CANCELAR_DO_OK[0], ok[1] + CANCELAR_DO_OK[1])
            return
        if fora:
            if time.time() - self.ultimo_convite > config.REENVIAR_CONVITE:
                self.convidar(principal, frame, [c.nome for c in fora])
            return
        if tela.nome in ("SALA_SEM_INSTANCIA", "SELECAO_INSTANCIA"):
            principal.fazer_cliques(tela, frame)  # escolhe Formigueiro / Normal
            return
        if tela.nome == "SALA_PRONTA" and prontas == len(self.convidadas):
            principal.log(f"{prontas} de {prontas} convidada(s) pronta(s): iniciando")
            for c in self.convidadas:
                c.na_luta = True  # na batalha: Confiar e passar a vez
            principal.fazer_cliques(tela, frame)
            time.sleep(random.uniform(*config.ESPERA_TRANSICAO))
        else:
            principal.log(f"{prontas} de {len(self.convidadas)} convidada(s) pronta(s)")

    @staticmethod
    def xis_dos_cartoes(frame):
        """Centros dos X pequenos dos cartões ocupados da sala, em ordem (linha, coluna).

        O 1º é o cartão de cima à esquerda (o da dona da sala)."""
        tpl = vision.carregar(XIS)
        h, w = tpl.shape[:2]
        res = cv2.matchTemplate(frame, tpl, cv2.TM_CCOEFF_NORMED)
        pontos = []
        for y, x in sorted(zip(*np.where(res >= 0.85)), key=lambda p: -res[p]):
            if all(abs(x - a) > 10 or abs(y - b) > 10 for a, b in pontos):
                pontos.append((int(x), int(y)))
        pontos.sort(key=lambda p: (p[1] // 20, p[0]))
        return [(x + w // 2, y + h // 2) for x, y in pontos]

    def sala_sozinha(self, principal):
        """Antes do estágio 1: a dona da sala expulsa as convidadas (X do cartão) e inicia
        sozinha, confirmando o aviso de lutar sozinha."""
        # A dona da sala expulsa as convidadas: X pequeno no cartão de cada uma.
        frame = self.olhar(principal)
        outros = self.xis_dos_cartoes(frame)[1:]  # o 1º (em cima à esquerda) é o dela
        if outros:
            for x, y in outros:
                principal.log("expulsando uma convidada da sala (X do cartão)")
                principal.driver.clicar(x, y)
                time.sleep(random.uniform(0.3, 0.5))
            time.sleep(0.4)
            frame = principal.driver.capturar()
            if self.xis_dos_cartoes(frame)[1:]:
                principal.log("ainda tem convidada na sala")
                principal.salvar_debug(frame, "expulsar")
                return  # só inicia sozinha quando todas tiverem saído
        tela = telas.identificar(frame, config.LIMIAR, ignorar=("SEM_FOCO",))
        if tela is None:
            return
        if tela.nome == "SALA_PRONTA" and contar_prontos(frame) > 0:
            principal.log("ainda tem convidada na sala; esperando ela sair")
            return
        if tela.cliques:
            principal.log("estágio 1: indo sozinha")
            principal.fazer_cliques(tela, frame)  # Início e, se aparecer, OK do Lembrete
            time.sleep(random.uniform(*config.ESPERA_TRANSICAO))

    def batalha_so_principal(self, vistas):
        """Modo 'só a principal joga'. Retorna True se cuidou da batalha neste passo.

        No começo de cada estágio, visita cada convidada uma vez (troca de tela + clique)
        e liga o Confiar dela. Depois fica na principal e joga só as vezes dela.
        """
        principal, frame_p, tela_p = vistas[0]
        if tela_p is not None and tela_p.nome in NA_SALA:
            for c in self.convidadas:
                c.confiou = False  # estágio novo: o Confiar precisa ser ligado de novo
                c.na_luta = False
            self.principal_jogou = False
            return False
        alguma_batalha = any(t is not None and t.batalha for _, _, t in vistas)
        if not alguma_batalha:
            return False
        if tela_p is not None and tela_p.batalha:
            self.na_batalha = True  # próximas voltas pelo caminho rápido
        if not self.principal_jogou:
            # A principal sempre joga primeiro; só depois o Confiar nas convidadas.
            if not principal.driver.na_frente():
                principal.driver.ativar_com_clique()
            frame = principal.driver.capturar()
            leitura = ler_batalha(frame)
            if leitura is None:
                return False
            if leitura.minha_vez:
                principal.driver.ativar_com_clique()
                principal.batalha.passo(principal.driver.capturar())
                self.principal_jogou = True
            else:
                time.sleep(0.3)
            return True
        # Convidadas que entraram nesta luta (marcadas ao iniciar). A janela delas pode estar
        # congelada mostrando a sala, então não dá para confiar no print antigo.
        em_batalha = {c for c, _, t in vistas if t is not None and t.batalha}
        for conta in self.convidadas:
            if conta.confiou or not (conta.na_luta or conta in em_batalha):
                continue  # convidada fora da luta (ex.: estágio 1, a principal vai sozinha)
            if not conta.driver.ativar_com_clique():
                continue
            frame = conta.driver.capturar()
            leitura = ler_batalha(frame)
            if leitura is None:
                continue  # batalha ainda carregando nesta janela: tenta na próxima volta
            if leitura.estagio == "FORMIGAS":
                # Estágio 1: na vez dela, anda até ficar em cima da principal, liga o Confiar
                # e passa o turno. Só dá para andar na própria vez: espera com a janela na frente.
                limite = time.time() + 40
                while not leitura.minha_vez and time.time() < limite:
                    time.sleep(0.3)
                    leitura = ler_batalha(conta.driver.capturar()) or leitura
                if not leitura.minha_vez:
                    conta.log("a vez dela não chegou; tento de novo depois")
                    return True
                conta.driver.ativar_com_clique()
                leitura = ler_batalha(conta.driver.capturar()) or leitura
                if leitura.eu:
                    conta.log("andando até ficar em cima da principal")
                    conta.batalha.ir_ate_principal(leitura)
                frame = conta.driver.capturar()
            pos = vision.achar(CONFIAR, frame, 0.8)
            if pos is None:
                conta.log("botão Confiar não apareceu (batalha ainda carregando?)")
                return True
            conta.log("ligando o Confiar (o jogo joga por esta conta)")
            conta.driver.clicar(*pos)
            conta.confiou = True
            time.sleep(0.5)
            passar = vision.achar(PASS_BATALHA, conta.driver.capturar(), LIMIAR_PASS)
            if passar:
                conta.log("passando o turno")
                conta.driver.clicar(*passar)
            return True
        # Todas no Confiar: fica na principal e joga a vez dela.
        if not principal.driver.na_frente():
            principal.driver.ativar_com_clique()
        frame = principal.driver.capturar()
        leitura = ler_batalha(frame)
        if leitura is None:
            return False
        if leitura.minha_vez:
            principal.driver.ativar_com_clique()
            principal.batalha.passo(principal.driver.capturar())
        else:
            time.sleep(0.3)
        return True

    def jogar_vez(self, conta, frame):
        """Joga o turno da conta, aprende a ordem das vezes e já prepara a próxima."""
        # Todo turno: passa para a janela dela e dá 1 clique antes de jogar.
        conta.driver.ativar_com_clique()
        frame = conta.driver.capturar()
        conta.batalha.passo(frame)
        if conta in self.ordem:
            # Já vi essa conta jogar: a ordem está completa até ela; a próxima é a seguinte.
            self.ordem = self.ordem[self.ordem.index(conta):] + self.ordem[:self.ordem.index(conta)]
            self.proxima = self.ordem[1] if len(self.ordem) > 1 else conta
        else:
            self.ordem.append(conta)
            self.proxima = None  # ainda aprendendo a ordem
        if self.proxima is not None:
            conta_prox = self.proxima
            conta_prox.log("próxima da fila: trazendo para frente")
            conta_prox.driver.com_foco()

    def esperar_vez(self, conta, segundos=20):
        """Espera a vez da conta com a janela dela na frente (ela desenha). True se jogou."""
        if not conta.driver.na_frente():
            conta.driver.com_foco()
        limite = time.time() + segundos
        while time.time() < limite:
            frame = conta.driver.capturar()
            leitura = ler_batalha(frame)
            if leitura is None:
                return False  # saiu da batalha (estágio novo ou fim)
            if leitura.minha_vez:
                self.jogar_vez(conta, frame)
                return True
            time.sleep(0.3)
        return False

    @staticmethod
    def esperar_imagem(conta, template, limiar, segundos):
        """Tira prints até o template aparecer (ou o tempo acabar). Retorna o último print."""
        limite = time.time() + segundos
        frame = conta.driver.capturar()
        while vision.achar(template, frame, limiar) is None and time.time() < limite:
            time.sleep(0.1)
            frame = conta.driver.capturar()
        return frame

    @staticmethod
    def olhar(conta):
        """Traz a janela para frente (ela volta a desenhar), tira o escurecido do
        'Clique no jogo' (duplo clique) e tira um print atualizado."""
        conta.driver.com_foco()
        time.sleep(random.uniform(0.3, 0.5))
        return conta.driver.capturar()

    def dar_pronto(self, conta):
        """Clica em Pronto na convidada. O botão fica no lugar do Início (ao lado do Troque)."""
        frame = self.olhar(conta)
        pos = vision.achar(TROQUE, frame, config.LIMIAR)
        if pos is None:
            return False
        conta.log("clicando em Pronto")
        conta.driver.clicar(pos[0] + 70, pos[1])
        conta.pronto = True
        time.sleep(random.uniform(*config.ESPERA_CLIQUE))
        return True

    def conferir_prontos(self, principal, frame):
        """Na janela da principal: todas as convidadas prontas? Se faltar, corrige.

        Para cada convidada: clica em Pronto de novo e volta à principal para contar.
        Se a conta cair, o clique tirou o Pronto dela: clica mais uma vez para recolocar.
        """
        total = len(self.convidadas)
        prontos = contar_prontos(frame)
        principal.log(f"{prontos} de {total} convidada(s) pronta(s)")
        if prontos >= total:
            principal.log("todas prontas: iniciando")
            return True
        for convidada in self.convidadas:
            if not self.dar_pronto(convidada):
                continue
            agora = contar_prontos(self.olhar(principal))
            if agora < prontos:
                convidada.log("o clique tirou o Pronto: clicando de novo")
                self.dar_pronto(convidada)
                agora = contar_prontos(self.olhar(principal))
            prontos = agora
            principal.log(f"{prontos} de {total} convidada(s) pronta(s)")
            if prontos >= total:
                principal.log("todas prontas: iniciando")
                return True
        return False

    @staticmethod
    def botoes_das_contas(frame):
        """Botões 'Convidar' que estão na mesma linha do nome de uma das minhas contas.

        Os nomes ficam em img/convidados/ (um recorte do nome por conta, como aparece na
        lista de amigos). Assim o bot nunca convida desconhecidos de outra aba.
        """
        tpl = vision.carregar(CONVIDAR)
        h, w = tpl.shape[:2]
        res = cv2.matchTemplate(frame, tpl, cv2.TM_CCOEFF_NORMED)
        botoes = []
        for y, x in sorted(zip(*np.where(res >= 0.85)), key=lambda p: -res[p]):
            if all(abs(y - b) > 8 or abs(x - a) > 20 for a, b in botoes):
                botoes.append((int(x), int(y)))
        achados = []
        for arq in sorted(PASTA_CONVIDADOS.glob("*.png")):
            nome = vision.achar(arq, frame, 0.9)
            if nome is None:
                continue
            # O botão da linha: mesma altura (até 10 px) e à direita do nome (até 250 px).
            linha = [(x, y) for x, y in botoes
                     if abs(y + h // 2 - nome[1]) <= 10 and 0 < x - nome[0] < 250]
            if linha:
                x, y = min(linha, key=lambda b: b[0] - nome[0])
                achados.append((arq.stem, (x + w // 2, y + h // 2)))
        return achados

    def convidar(self, conta, frame, nomes):
        conta.log(f"convidando ({', '.join(nomes)} fora da sala)")
        self.ultimo_convite = time.time()
        # A janela do Convide pode ter ficado aberta: clicar em Convide de novo a fecharia.
        if vision.achar(ABA_AMIGOS, frame, config.LIMIAR) is None:
            if not conta.clicar_em(CONVIDE, frame):
                conta.log("botão Convide não encontrado")
                return
        if not conta.clicar_em(ABA_AMIGOS):
            conta.log("aba Amigos não apareceu")
            conta.salvar_debug(conta.driver.capturar(), "sem_aba_amigos")
            return
        # Segue assim que a lista de amigos mostrar os botões Convidar (até 1,5 s).
        botoes = self.botoes_das_contas(self.esperar_imagem(conta, CONVIDAR, 0.85, 1.5))
        if not botoes:
            conta.log("nenhuma das minhas contas na lista (aba errada?): não convidei ninguém")
            conta.salvar_debug(conta.driver.capturar(), "lista_convite")
            return
        for nome, (x, y) in botoes:
            conta.log(f"convidando {nome}")
            conta.driver.clicar(x, y)
            time.sleep(0.5)
        # Visita cada convidada enquanto o convite vale (12 s). Janela coberta fica congelada
        # e não mostra o convite: trazê-la para frente a faz desenhar de novo.
        limite = time.time() + 12
        for convidada in self.convidadas:
            trazer_para_frente(convidada.driver.hwnd)
            while time.time() < limite:
                if convidada.aceitar_convite():
                    break
                time.sleep(0.3)

    def rodar(self):
        avisar("Grupo iniciado (Ctrl+C para parar; mouse no canto da tela também para)")
        sucesso = True
        try:
            verificacao.preparar()
            while True:
                self.passo()
                time.sleep(0.3)
        except KeyboardInterrupt:
            avisar("parado pelo usuário")
        except pyautogui.FailSafeException:
            avisar("parado: mouse no canto da tela")
        except verificacao.ErroInfraestrutura as exc:
            sucesso = False
            avisar(f"parado: {exc}")
        finally:
            # Soltar as teclas mesmo com o mouse no canto (a trava de segurança barraria).
            pyautogui.FAILSAFE = False
            for tecla in (config.TECLA_ATIRAR, config.TECLA_ESQUERDA, config.TECLA_DIREITA,
                          config.TECLA_ANGULO_SOBE, config.TECLA_ANGULO_DESCE):
                pyautogui.keyUp(tecla)
            pyautogui.FAILSAFE = True
        return sucesso


if __name__ == "__main__":
    import sys
    # python -m src.grupo [principal] [convidada ...]   (números das janelas; ver listar_janelas)
    numeros = [int(a) for a in sys.argv[1:]]
    sys.exit(0 if Grupo(numeros[0] if numeros else None, numeros[1:]).rodar() else 1)
