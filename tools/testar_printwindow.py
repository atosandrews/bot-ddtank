"""Testa se dá para tirar print das janelas do jogo mesmo cobertas (PrintWindow), sem clicar.
Uso: python tools/testar_printwindow.py
Lista as janelas do jogo, salva logs/printwindow_<hwnd>.png e diz se o print veio preto.
"""
import sys
from pathlib import Path

import cv2

RAIZ = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(RAIZ))
from src import config, telas
from src.drivers import DriverJanela
from src.drivers.tela import ativar_dpi, listar_janelas

ativar_dpi()
janelas = [(h, t) for h, t in listar_janelas() if config.TITULO_JANELA.lower() in t.lower()]
print(f"Janelas com '{config.TITULO_JANELA}':", janelas)
(RAIZ / "logs").mkdir(exist_ok=True)
for hwnd, titulo in janelas:
    frame = DriverJanela(hwnd).capturar()
    arq = RAIZ / "logs" / f"printwindow_{hwnd}.png"
    cv2.imwrite(str(arq), frame)
    tela = telas.identificar(frame, config.LIMIAR)
    print(f"{hwnd}: {frame.shape[1]}x{frame.shape[0]}, brilho médio {frame.mean():.0f} "
          f"({'PRETO' if frame.mean() < 5 else 'ok'}), tela reconhecida: {tela.nome if tela else None} -> {arq.name}")
