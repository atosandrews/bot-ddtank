"""Espera a minha vez na batalha (PASS na tela), salva o print e mostra a leitura. Não aperta nada.
Uso: python tools/capturar_minha_vez.py [segundos]
"""
import sys
import time
from pathlib import Path

import cv2

RAIZ = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(RAIZ))
from src import batalha, config
from src.drivers import DriverTela

driver = DriverTela.da_janela(config.TITULO_JANELA)
limite = time.time() + (float(sys.argv[1]) if len(sys.argv) > 1 else 90)
while time.time() < limite:
    frame = driver.capturar()
    leitura = batalha.ler(frame)
    if leitura and leitura.minha_vez:
        (RAIZ / "logs").mkdir(exist_ok=True)
        cv2.imwrite(str(RAIZ / "logs" / "minha_vez.png"), frame)
        print("Minha vez! Eu:", leitura.eu, "| Inimigos:", leitura.inimigos)
        print("Print salvo em logs/minha_vez.png")
        break
    time.sleep(0.3)
else:
    print("Minha vez não apareceu no tempo.")
