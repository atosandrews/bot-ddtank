"""Adaptador do SDK local; não captura tela nem envia comandos ao jogo."""
from dataclasses import dataclass
from functools import lru_cache
import re

import cv2
import numpy as np


class ErroInfraestrutura(RuntimeError):
    """Instalação, modelos ou janela indisponíveis; repetir não corrigiria o erro."""


@dataclass(frozen=True)
class Candidato:
    texto: str
    confianca: float
    modelo: str
    variante: str


class OcrLocal:
    def __init__(self, fabrica=None):
        try:
            if fabrica is None:
                from ddddocr import DdddOcr
                fabrica = DdddOcr
            self.modelos = [(nome, fabrica(show_ad=False, use_gpu=False, beta=beta))
                            for nome, beta in (("padrao", False), ("beta", True))]
            for _, modelo in self.modelos:
                modelo.set_ranges("0123456789")
        except Exception as exc:
            raise ErroInfraestrutura(
                "OCR indisponível. No diretório bot-ddtank, instale requirements.txt "
                "no Python do .venv e confira os modelos ONNX do projeto ../ddddocr."
            ) from exc

    @staticmethod
    def variantes(imagem):
        if imagem is None or imagem.size == 0:
            return []
        cinza = cv2.cvtColor(imagem, cv2.COLOR_BGR2GRAY) if imagem.ndim == 3 else imagem
        return [("original", imagem),
                ("cinza_2x", cv2.resize(cinza, None, fx=2, fy=2, interpolation=cv2.INTER_CUBIC))]

    def reconhecer(self, imagem, limiar=0.90):
        unicos = {}
        self.ultimas_leituras = []
        for variante, pixels in self.variantes(imagem):
            ok, png = cv2.imencode(".png", pixels)
            if not ok:
                continue
            for nome, modelo in self.modelos:
                try:
                    resposta = modelo.classification(png.tobytes(), probability=True)
                    texto = resposta.get("text", "")
                    confianca = float(resposta.get("confidence", 0.0))
                except Exception as exc:
                    raise ErroInfraestrutura(f"Falha de inferência no modelo {nome}: {exc}") from exc
                self.ultimas_leituras.append({"texto": texto, "confianca": confianca,
                                             "modelo": nome, "variante": variante})
                if (isinstance(texto, str) and re.fullmatch(r"[0-9]{4}", texto)
                        and np.isfinite(confianca) and limiar <= confianca <= 1.0):
                    candidato = Candidato(texto, confianca, nome, variante)
                    if texto not in unicos or confianca > unicos[texto].confianca:
                        unicos[texto] = candidato
        return sorted(unicos.values(), key=lambda c: c.confianca, reverse=True)

    def ler_tempo(self, imagem):
        # O contador é vermelho. Isolar a cor evita reconhecer o fundo do título.
        b, g, r = cv2.split(imagem)
        mascara = (r.astype(float) > g.astype(float) * 1.5) & (r > 110) & (r > b)
        if not mascara.any():
            return None
        pixels = np.where(mascara, 0, 255).astype(np.uint8)
        pixels = cv2.copyMakeBorder(pixels, 4, 4, 4, 4, cv2.BORDER_CONSTANT, value=255)
        ok, png = cv2.imencode(".png", pixels)
        if not ok:
            return None
        try:
            texto = self.modelos[0][1].classification(png.tobytes())
        except Exception as exc:
            raise ErroInfraestrutura(f"Falha ao ler contador: {exc}") from exc
        if isinstance(texto, str) and re.fullmatch(r"[0-9]{1,2}", texto):
            valor = int(texto)
            if 0 <= valor <= 30:
                return valor
        return None


@lru_cache(maxsize=1)
def obter_ocr():
    """Uma instância dos dois modelos por processo, compartilhada entre contas."""
    return OcrLocal()
