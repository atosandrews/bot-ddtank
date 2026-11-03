"""Diagnóstico: lê captchas do jogo (PNG RGBA do pacote) com o OCR do bot-ddtank em vários preparos."""
import sys

import cv2
import numpy as np

sys.path[:0] = ["/ddddocr", "/bot-ddtank"]
from src.verificacao_ocr import obter_ocr, palpite  # noqa: E402

ocr = obter_ocr()
for caminho in sys.argv[1:]:
    bruta = cv2.imread(caminho, cv2.IMREAD_UNCHANGED)
    print(caminho, bruta.shape, "alfa min/max:", bruta[..., 3].min(), bruta[..., 3].max())
    alfa = bruta[..., 3:4].astype(np.float32) / 255
    branca = (bruta[..., :3] * alfa + 255 * (1 - alfa)).astype(np.uint8)
    preparos = {"sem alfa (como hoje)": cv2.imread(caminho, cv2.IMREAD_COLOR), "fundo branco": branca}
    for escala in (1.37, 2.0):
        preparos[f"fundo branco x{escala}"] = cv2.resize(branca, None, fx=escala, fy=escala, interpolation=cv2.INTER_CUBIC)
    for nome, img in preparos.items():
        decisao = ocr.reconhecer(img, 0.90)
        leituras = sorted({(l["texto"], round(l["confianca"], 2), l["modelo"], l["variante"]) for l in ocr.ultimas_leituras}, key=lambda x: -x[1])[:6]
        chute = palpite(ocr.ultimas_leituras)
        print(f"  {nome:24} decidir={[c.texto for c in decisao]} palpite={chute and chute.texto} leituras={leituras}")
