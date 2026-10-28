"""Grava prints da janela do jogo a cada N segundos em logs/gravacao/, sem apertar nada.
Uso: python tools/gravar_prints.py [segundos_total] [intervalo]
Para cada print, mostra se o bot acha a batalha, a minha vez (PASS) e o estágio.
"""
import sys
import time
from pathlib import Path

import cv2

RAIZ = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(RAIZ))
from src import batalha, config, vision
from src.drivers import DriverTela

total = float(sys.argv[1]) if len(sys.argv) > 1 else 60
intervalo = float(sys.argv[2]) if len(sys.argv) > 2 else 2
pasta = RAIZ / "logs" / "gravacao"
pasta.mkdir(parents=True, exist_ok=True)
driver = DriverTela.da_janela(config.TITULO_JANELA)
inicio = time.time()
i = 0
while time.time() - inicio < total:
    frame = driver.capturar()
    leitura = batalha.ler(frame)
    _, conf_pass = vision.procurar(batalha.PASS, frame)
    nome = f"{i:03d}.png"
    cv2.imwrite(str(pasta / nome), frame)
    if leitura:
        print(f"{nome} batalha, estágio {leitura.estagio}, minha vez {leitura.minha_vez} (PASS {conf_pass:.2f})", flush=True)
    else:
        print(f"{nome} fora da batalha", flush=True)
    i += 1
    time.sleep(intervalo)
