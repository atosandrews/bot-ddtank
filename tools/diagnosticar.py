"""Tira um print da janela do jogo e mostra o que o bot reconhece nela, sem clicar.
Uso: python tools/diagnosticar.py "TITULO"
"""
import sys
from pathlib import Path

import cv2

RAIZ = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(RAIZ))
from src import config, telas, vision
from src.drivers import DriverTela

driver = DriverTela.da_janela(sys.argv[1] if len(sys.argv) > 1 else config.TITULO_JANELA)
frame = driver.capturar()
print("Região da janela:", driver.regiao)
print("Tamanho do print:", frame.shape[1], "x", frame.shape[0])
for tela in telas.TELAS:
    pos_r, conf_r = vision.procurar(tela.reconhecer, frame)
    pos_c, conf_c = vision.procurar(tela.cliques[0].alvo, frame) if tela.cliques else (None, 0.0)
    print(f"  {tela.nome:<18} reconhecer {conf_r:.2f} em {pos_r} | clicar {conf_c:.2f} em {pos_c}")
tela = telas.identificar(frame, config.LIMIAR)
print("Tela reconhecida:", tela.nome if tela else None)
(RAIZ / "logs").mkdir(exist_ok=True)
cv2.imwrite(str(RAIZ / "logs" / "diagnostico.png"), frame)
print("Print salvo em logs/diagnostico.png")
