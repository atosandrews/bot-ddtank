"""Testa se um template é encontrado na janela e salva um print com o resultado.
Uso: python tools/testar_template.py img/botao_iniciar.png "TITULO"
"""
import sys

import cv2

sys.path.insert(0, ".")
from src import vision
from src.drivers import DriverTela

template, titulo = sys.argv[1], sys.argv[2]
frame = DriverTela.da_janela(titulo).capturar()
pos = vision.achar(template, frame, limiar=0.7)
print("Encontrado em:", pos)
if pos:
    cv2.circle(frame, pos, 15, (0, 0, 255), 3)
cv2.imwrite("logs/teste_template.png", frame)
print("Print salvo em logs/teste_template.png")
