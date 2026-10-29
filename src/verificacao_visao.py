"""Localização do quadro e dos controles no perfil visual nativo de 517x314."""
from dataclasses import dataclass
import hashlib
import json
from pathlib import Path

import cv2
import numpy as np

from . import config, vision
from .verificacao_ocr import ErroInfraestrutura

PASTA = Path(__file__).resolve().parent.parent / "img" / "verificacao"
TITULO = PASTA.parent / "codigo_verificacao.png"


class QuadroInvalido(ValueError):
    """Quadro visível, mas incompleto, ambíguo ou fora do perfil validado."""


@dataclass
class Quadro:
    origem: tuple
    codigo: np.ndarray
    tempo: np.ndarray
    identidade: str
    teclas: dict
    confirmar: tuple
    resetar: tuple
    ocupados: int


class VisaoVerificacao:
    def __init__(self, pasta=PASTA):
        try:
            self.perfil = json.loads((pasta / "perfil.json").read_text(encoding="utf-8"))
            self.titulo = vision.carregar(TITULO)
            self.templates = {nome: vision.carregar(pasta / f"{nome}.png")
                              for nome in self.perfil["templates"]}
        except (OSError, ValueError, KeyError, TypeError) as exc:
            raise ErroInfraestrutura(f"Perfil/templates de verificação indisponíveis: {exc}") from exc

    def ancora(self, frame):
        if frame is None or frame.ndim != 3 or frame.size == 0:
            return None
        h, w = self.titulo.shape[:2]
        if frame.shape[0] < h or frame.shape[1] < w:
            return None
        resposta = cv2.matchTemplate(frame, self.titulo, cv2.TM_CCOEFF_NORMED)
        _, valor, _, pos = cv2.minMaxLoc(resposta)
        return pos if valor >= config.VERIFICACAO_LIMIAR_TITULO else None

    @staticmethod
    def _recortar(frame, caixa):
        x, y, w, h = caixa
        return frame[y:y + h, x:x + w]

    def _alvo(self, area, nome, deslocamento):
        template = self.templates[nome]
        h, w = template.shape[:2]
        if h > area.shape[0] or w > area.shape[1]:
            raise QuadroInvalido(f"Região pequena para {nome}")
        scores = cv2.matchTemplate(area, template, cv2.TM_CCOEFF_NORMED)
        _, melhor, _, (x, y) = cv2.minMaxLoc(scores)
        # Descartar vizinhança do mesmo alvo antes de procurar um segundo destino.
        restantes = scores.copy()
        restantes[max(0, y-h//2):y+h//2+1, max(0, x-w//2):x+w//2+1] = -1
        segundo = float(restantes.max())
        if (melhor < config.VERIFICACAO_LIMIAR_TECLAS
                or melhor - segundo < config.VERIFICACAO_MARGEM_TECLAS):
            raise QuadroInvalido(f"Destino ambíguo/ausente: {nome}")
        return x + w//2 + deslocamento[0], y + h//2 + deslocamento[1]

    def analisar(self, frame):
        pos = self.ancora(frame)
        if pos is None:
            return None
        ax, ay = self.perfil["anchor"]
        ox, oy = pos[0] - ax, pos[1] - ay
        w, h = self.perfil["size"]
        if ox < 0 or oy < 0 or ox+w > frame.shape[1] or oy+h > frame.shape[0]:
            raise QuadroInvalido("Quadro cortado ou escala incompatível")
        popup = frame[oy:oy+h, ox:ox+w]
        teclado = self._recortar(popup, self.perfil["keyboard"])
        kx, ky, _, _ = self.perfil["keyboard"]
        teclas = {digito: self._alvo(teclado, digito, (ox+kx, oy+ky))
                  for digito in "0123456789"}
        for a, ponto in teclas.items():
            for b, outro in teclas.items():
                if a != b and np.linalg.norm(np.subtract(ponto, outro)) < 10:
                    raise QuadroInvalido("Dois números apontam para o mesmo botão")
        controles = self._recortar(popup, self.perfil["controls"])
        cx, cy, _, _ = self.perfil["controls"]
        confirmar = self._alvo(controles, "confirmar", (ox+cx, oy+cy))
        resetar = self._alvo(controles, "resetar", (ox+cx, oy+cy))
        casas = []
        for caixa in self.perfil["slots"]:
            casa = self._recortar(popup, caixa)
            casas.append(int(np.count_nonzero(cv2.cvtColor(casa, cv2.COLOR_BGR2GRAY) < 120)) >= 4)
        ocupados = sum(casas)
        if casas != [True]*ocupados + [False]*(4-ocupados):
            raise QuadroInvalido("Casas de entrada inconsistentes")
        codigo = self._recortar(popup, self.perfil["code"]).copy()
        identidade = hashlib.sha256(codigo.tobytes()).hexdigest()
        return Quadro((ox, oy), codigo, self._recortar(popup, self.perfil["timer"]).copy(),
                      identidade, teclas, confirmar, resetar, ocupados)
