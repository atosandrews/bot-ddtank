import random
import time
from datetime import datetime

import cv2

from . import vision


class Bot:
    def __init__(self, driver, nome="bot1"):
        self.driver = driver
        self.nome = nome
        self.estado = "LOBBY"

    def log(self, msg):
        print(f"[{datetime.now():%H:%M:%S}] [{self.nome}] [{self.estado}] {msg}")

    def salvar_debug(self, frame):
        cv2.imwrite(f"logs/{self.nome}_{datetime.now():%H%M%S}.png", frame)

    def clicar_template(self, frame, template, proximo_estado):
        pos = vision.achar(template, frame)
        if pos:
            self.log(f"achou {template} em {pos}")
            self.driver.clicar(*pos)
            self.estado = proximo_estado
            return True
        return False

    def passo(self):
        frame = self.driver.capturar()

        if self.estado == "LOBBY":
            self.clicar_template(frame, "img/botao_dungeon.png", "SALA")
        elif self.estado == "SALA":
            self.clicar_template(frame, "img/botao_iniciar.png", "BATALHA")
        elif self.estado == "BATALHA":
            pass  # TODO: detectar turno, mirar, atirar
        elif self.estado == "LOOT":
            pass  # TODO: coletar recompensa e voltar ao LOBBY

        time.sleep(random.uniform(0.3, 0.8))

    def rodar(self):
        self.log("iniciado (Ctrl+C para parar)")
        try:
            while True:
                self.passo()
        except KeyboardInterrupt:
            self.log("parado")
