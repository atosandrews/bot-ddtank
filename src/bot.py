import random
import time
from datetime import datetime
from pathlib import Path

import cv2
import pyautogui

from . import config, telas, verificacao, vision
from .batalha import Batalha

LOGS = Path(__file__).resolve().parent.parent / "logs"


class Bot:
    def __init__(self, driver, nome="bot1"):
        self.driver = driver
        self.nome = nome
        self.estado = "INICIO"
        self.repeticoes = 0
        self.sem_tela = 0
        self.batalha = Batalha(driver, self.log, self.salvar_debug)

    def log(self, msg):
        print(f"[{datetime.now():%H:%M:%S}] [{self.nome}] [{self.estado}] {msg}", flush=True)

    def salvar_debug(self, frame, motivo):
        LOGS.mkdir(exist_ok=True)
        arquivo = LOGS / f"{self.nome}_{datetime.now():%H%M%S}_{motivo}.png"
        cv2.imwrite(str(arquivo), frame)
        self.log(f"print salvo em {arquivo}")

    def esperar_alvo(self, alvo, frame):
        """Procura o alvo até ele aparecer ou o tempo acabar. Retorna (pos ou None, último frame)."""
        limite = time.time() + config.ESPERA_ALVO
        while True:
            pos = vision.achar(alvo, frame, config.LIMIAR)
            if pos or time.time() >= limite:
                return pos, frame
            time.sleep(0.3)
            frame = self.driver.capturar()

    def passo(self):
        """Executa um ciclo. Retorna False quando o fluxo conhecido terminou."""
        frame = self.driver.capturar()
        if verificacao.tem_codigo(frame):
            verificacao.tratar(self.driver, self.nome)
            return True
        tela = telas.identificar(frame, config.LIMIAR)

        if tela is None:
            self.sem_tela += 1
            if self.sem_tela == config.SEM_TELA_MAX:
                self.log("nenhuma tela conhecida há um tempo")
                self.salvar_debug(frame, "desconhecida")
            # O balão de dica que aparece com o mouse em cima pode esconder o alvo.
            self.driver.afastar_mouse()
            time.sleep(1)
            return True
        self.sem_tela = 0

        # Saiu da batalha depois de jogar o último estágio: a instância acabou.
        if (config.PARAR_DEPOIS_DA_INSTANCIA and not tela.batalha and tela.nome != "SEM_FOCO"
                and self.batalha.estagio == config.ULTIMO_ESTAGIO):
            self.log(f"instância terminada (passou do estágio {config.ULTIMO_ESTAGIO}); parando")
            self.salvar_debug(frame, "fim_da_instancia")
            return False

        if tela.nome == self.estado:
            self.repeticoes += 1
            if self.repeticoes == config.REPETICOES_MAX and not tela.batalha:
                self.log("a tela não mudou depois de vários cliques (clique no lugar errado?)")
                self.salvar_debug(frame, "travado")
        else:
            self.estado = tela.nome
            self.repeticoes = 0
            self.log("tela reconhecida")
            if tela.batalha:
                ajuste = "eu ajusto" if config.AJUSTAR_ANGULO else "ajuste à mão com ↑/↓"
                self.log(f"batalha: ângulo {ajuste}; começo em {config.PAUSA_INICIO_BATALHA} s")
                time.sleep(config.PAUSA_INICIO_BATALHA)
                return True

        if tela.batalha:
            self.batalha.passo(frame)
            return True

        for clique in tela.cliques:
            pos, frame = self.esperar_alvo(clique.alvo, frame)
            if pos is None:
                if clique.opcional:
                    self.log(f"{clique.alvo.name} não apareceu (opcional, seguindo)")
                    continue
                self.log(f"alvo {clique.alvo.name} não encontrado")
                return True

            x, y = pos[0] + clique.deslocamento[0], pos[1] + clique.deslocamento[1]
            self.log(f"clicando em {clique.nome or clique.alvo.name} ({x}, {y})")
            if config.SALVAR_PRINTS:
                self.salvar_debug(frame, f"antes_{clique.alvo.stem}")
            self.driver.clicar(x, y)
            time.sleep(random.uniform(*config.ESPERA_CLIQUE))
            frame = self.driver.capturar()

        if tela.final:
            if config.SALVAR_PRINTS:
                time.sleep(3)
                self.salvar_debug(self.driver.capturar(), f"depois_{tela.nome}")
            self.log("fim do fluxo da especificação (próximos estados a definir)")
            return False

        time.sleep(random.uniform(*config.ESPERA_TRANSICAO))
        return True

    def rodar(self):
        self.log("iniciado (Ctrl+C para parar; mouse no canto da tela também para)")
        sucesso = True
        try:
            verificacao.preparar()
            while self.passo():
                pass
        except KeyboardInterrupt:
            self.log("parado pelo usuário")
        except pyautogui.FailSafeException:
            self.log("parado: mouse no canto da tela")
        except verificacao.ErroInfraestrutura as exc:
            sucesso = False
            self.log(f"parado: {exc}")
        finally:
            # Nunca deixar uma tecla presa se o bot parar no meio de um tiro (mesmo com o
            # mouse no canto, que dispara a trava de segurança).
            pyautogui.FAILSAFE = False
            for tecla in (config.TECLA_ATIRAR, config.TECLA_ESQUERDA, config.TECLA_DIREITA,
                          config.TECLA_ANGULO_SOBE, config.TECLA_ANGULO_DESCE):
                pyautogui.keyUp(tecla)
            pyautogui.FAILSAFE = True
        self.log("encerrado")
        return sucesso
