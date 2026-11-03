"""Leitor de captcha para o bot em Go, usando o OCR validado do bot-ddtank.

Protocolo (sem HTTP): uma linha JSON por pedido na entrada, uma linha JSON por resposta.
  pedido:   {"image": "<PNG em base64>"}
  resposta: {"text": "1234", "confidence": 0.97, "source": "decidir" | "palpite"}
            ou {"error": "motivo"}

Mesma regra de src/verificacao.py: primeiro decidir() com VERIFICACAO_LIMIAR_OCR; sem
certeza e com VERIFICACAO_ENVIAR_PALPITE, o melhor palpite. Um código já devolvido para a
mesma imagem não volta: se o servidor reenviar a imagem, sai o próximo candidato.
"""
import base64
import hashlib
import json
import os
import sys
import time
from pathlib import Path

import cv2
import numpy as np

sys.path[:0] = ["/ddddocr", "/bot-ddtank"]

from src import config  # noqa: E402
from src.verificacao_ocr import obter_ocr, palpite  # noqa: E402

LIMIAR = config.VERIFICACAO_LIMIAR_OCR
ENVIAR_PALPITE = config.VERIFICACAO_ENVIAR_PALPITE
ENVIADOS = {}  # sha256 da imagem -> códigos já devolvidos
# Banco de captchas do jogo: imagem (<sha256>.png) e uma linha JSON por resposta em respostas.jsonl.
# O resultado (aceito/recusado) é anotado pelo script do grupo e juntado pelo painel.
BANCO = Path(os.environ.get("CAPTCHA_BANCO", "/ws/run/captchas"))


def registrar(chave, dados, resposta, leituras):
    try:
        BANCO.mkdir(parents=True, exist_ok=True)
        imagem = BANCO / f"{chave}.png"
        if not imagem.exists():
            imagem.write_bytes(dados)
        linha = {"sha256": chave, "hora": time.strftime("%Y-%m-%d %H:%M:%S"), **resposta,
                 "leituras": [[l["texto"], round(float(l["confianca"]), 3), l["modelo"], l["variante"]]
                              for l in leituras]}
        with (BANCO / "respostas.jsonl").open("a", encoding="utf-8") as arquivo:
            arquivo.write(json.dumps(linha, ensure_ascii=False) + "\n")
    except OSError as erro:
        print(f"banco de captchas indisponível: {erro}", file=sys.stderr, flush=True)


def forcar_troca(leituras, ja_enviados):
    """Palpite de 4 dígitos quando nenhuma leitura tem 4: no jogo sem cliente o prazo não troca
    a imagem, só 4 dígitos errados trocam. Dígitos sobrepostos fazem os modelos lerem 3; o
    palpite duplica um dígito da leitura de 3 com mais apoio (267 -> 2267, 2667, 2677)."""
    apoio = {}
    for leitura in leituras:
        texto = leitura.get("texto")
        if isinstance(texto, str) and len(texto) == 3 and texto.isdigit():
            apoio[texto] = apoio.get(texto, 0.0) + float(leitura.get("confianca") or 0.0)
    opcoes = []
    for texto in sorted(apoio, key=apoio.get, reverse=True):
        opcoes += [texto[:i] + texto[i] + texto[i:] for i in range(len(texto))]
    opcoes += [str(d) * 4 for d in range(10)]
    return next((o for o in opcoes if o not in ja_enviados), None)


def responder(pedido):
    dados = base64.b64decode(pedido["image"], validate=True)
    imagem = cv2.imdecode(np.frombuffer(dados, np.uint8), cv2.IMREAD_COLOR)
    if imagem is None:
        return {"error": "imagem inválida"}

    chave = hashlib.sha256(dados).hexdigest()
    resposta = escolher(chave, imagem)
    registrar(chave, dados, resposta, getattr(obter_ocr(), "ultimas_leituras", ()))
    return resposta


def escolher(chave, imagem):
    ja_enviados = ENVIADOS.setdefault(chave, set())
    ocr = obter_ocr()
    candidatos = [c for c in ocr.reconhecer(imagem, LIMIAR) if c.texto not in ja_enviados]
    fonte = "decidir"
    if not candidatos and ENVIAR_PALPITE:
        escolhido = palpite(getattr(ocr, "ultimas_leituras", ()), ja_enviados)
        candidatos = [escolhido] if escolhido else []
        fonte = "palpite"
    if not candidatos and ENVIAR_PALPITE:
        texto = forcar_troca(getattr(ocr, "ultimas_leituras", ()), ja_enviados)
        if texto:
            ja_enviados.add(texto)
            return {"text": texto, "confidence": 0.0, "source": "troca"}
    if not candidatos:
        return {"error": "nenhum código de 4 dígitos com apoio suficiente"}

    escolhido = candidatos[0]
    ja_enviados.add(escolhido.texto)
    confianca = min(1.0, float(escolhido.confianca))
    return {"text": escolhido.texto, "confidence": confianca, "source": fonte}


def main():
    obter_ocr()  # carrega os dois modelos antes do primeiro captcha
    print(json.dumps({"ready": True}), flush=True)
    for linha in sys.stdin:
        try:
            resposta = responder(json.loads(linha))
        except Exception as exc:  # noqa: BLE001 - o bot precisa de uma resposta por pedido
            resposta = {"error": f"{type(exc).__name__}: {exc}"}
        print(json.dumps(resposta), flush=True)


main()
