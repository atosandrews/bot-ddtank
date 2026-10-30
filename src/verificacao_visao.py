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
AVISO_FOCO = PASTA.parent / "clique_no_jogo.png"  # "Clique no jogo, para continuar jogando"


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
    centro_roda: tuple = None  # centro do círculo dos botões, em coordenadas da janela
    fase: float = None  # ângulo de partida da roda (graus, módulo 36) neste print
    repouso: tuple = None  # onde o mouse espera, fora do teclado (coordenadas da janela)


class VisaoVerificacao:
    def __init__(self, pasta=PASTA):
        try:
            self.perfil = json.loads((pasta / "perfil.json").read_text(encoding="utf-8"))
            self.titulo = vision.carregar(TITULO)
            self.templates = {nome: vision.carregar(pasta / f"{nome}.png")
                              for nome in self.perfil["templates"]}
        except (OSError, ValueError, KeyError, TypeError) as exc:
            raise ErroInfraestrutura(f"Perfil/templates de verificação indisponíveis: {exc}") from exc
        self._cache_roda = {}

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

    def fase_da_roda(self, teclado):
        """Ângulo de partida da roda (graus, módulo 36) neste print.

        Os botões ficam num círculo (centro e raio do perfil) a 36° um do outro; só esse
        ângulo muda. É estimado pelos botões pequenos e soltos (os grandes de baixo encostam
        um no outro e na moldura e não servem para isso).
        """
        roda = self.perfil["ring"]
        cx, cy = roda["center"]
        raio = roda["radius"]
        hsv = cv2.cvtColor(teclado, cv2.COLOR_BGR2HSV)
        mascara = (hsv[..., 2] < roda["dark_max"]).astype(np.uint8) * 255
        borda = roda["border"]
        mascara[:borda], mascara[-borda:], mascara[:, :borda], mascara[:, -borda:] = 0, 0, 0, 0
        mascara = cv2.morphologyEx(mascara, cv2.MORPH_CLOSE, np.ones((5, 5), np.uint8))
        n, _, estat, centros = cv2.connectedComponentsWithStats(mascara)
        resto = []
        for i in range(1, n):
            area, largura = estat[i, cv2.CC_STAT_AREA], estat[i, cv2.CC_STAT_WIDTH]
            x, y = centros[i]
            if (roda["area_min"] <= area <= roda["area_max"] and largura <= roda["width_max"]
                    and abs(np.hypot(x - cx, y - cy) - raio) <= roda["radius_tol"]):
                resto.append(np.degrees(np.arctan2(y - cy, x - cx)) % 36)
        if len(resto) < roda["min_buttons"]:
            raise QuadroInvalido(f"Teclado não reconhecido ({len(resto)} botões soltos)")
        # Média circular com período de 36°.
        fases = np.radians(np.array(resto) * 10)
        inicio = np.degrees(np.arctan2(np.sin(fases).mean(), np.cos(fases).mean())) / 10 % 36
        desvio = [min(abs(a - inicio) % 36, 36 - abs(a - inicio) % 36) for a in resto]
        if max(desvio) > roda["angle_tol"]:
            raise QuadroInvalido("Botões fora do círculo esperado")
        return float(inicio)

    def _posicoes_da_roda(self, teclado):
        """As 10 posições dos botões, em ordem horária na tela."""
        roda = self.perfil["ring"]
        cx, cy = roda["center"]
        raio = roda["radius"]
        inicio = self.fase_da_roda(teclado)
        angulos = sorted((inicio + 36 * k) % 360 for k in range(10))
        # Ordem horária na tela (y para baixo): ângulo crescente a partir de -180°.
        angulos = sorted(((a + 180) % 360) - 180 for a in angulos)
        return [(cx + raio * np.cos(np.radians(a)), cy + raio * np.sin(np.radians(a)))
                for a in angulos]

    def _nota(self, teclado, digito, x, y, escala):
        """Maior correlação do número `digito` em volta de (x, y), perto da escala dada."""
        melhor = -1.0
        for fator in np.linspace(escala * 0.8, escala * 1.25, 8):
            tpl = cv2.resize(self.templates[digito], None, fx=fator, fy=fator,
                             interpolation=cv2.INTER_AREA if fator < 1 else cv2.INTER_CUBIC)
            h, w = tpl.shape[:2]
            if min(h, w) < 6:
                continue
            x0, y0 = int(max(0, x - w / 2 - 6)), int(max(0, y - h / 2 - 6))
            regiao = teclado[y0:y0 + h + 12, x0:x0 + w + 12]
            if regiao.shape[0] < h or regiao.shape[1] < w:
                continue
            melhor = max(melhor, cv2.minMaxLoc(cv2.matchTemplate(regiao, tpl, cv2.TM_CCOEFF_NORMED))[1])
        return melhor

    def teclas_pela_roda(self, teclado, deslocamento):
        """Números da roda: posições pelo círculo e rotação pela soma das 10 leituras.

        Os números seguem em ordem decrescente no sentido horário (conferido nos dois
        captchas reais disponíveis); só a rotação muda. Cada número tem o tamanho que o
        botão teria na posição da referência; a escala vem da altura (botões de baixo
        são maiores). Sem folga clara entre a melhor e a segunda rotação, não clica.
        """
        chave = hashlib.sha256(teclado.tobytes()).hexdigest()
        if chave in self._cache_roda:
            mapa = self._cache_roda[chave]
        else:
            roda = self.perfil["ring"]
            posicoes = self._posicoes_da_roda(teclado)
            altura = lambda y: roda["size_at_top"] + (y - roda["top_y"]) * roda["size_per_px"]
            notas = np.zeros((10, 10))
            for i, (x, y) in enumerate(posicoes):
                for d in range(10):
                    _, ry = roda["reference"][str(d)]
                    notas[i, d] = self._nota(teclado, str(d), x, y, altura(y) / altura(ry))
            totais = sorted(((sum(notas[i, (r - i) % 10] for i in range(10)), r) for r in range(10)),
                            reverse=True)
            (melhor, r), (segundo, _) = totais[0], totais[1]
            if melhor / 10 < roda["mean_min"] or melhor - segundo < roda["margin_min"]:
                raise QuadroInvalido(f"Roda ambígua (média {melhor / 10:.2f}, folga {melhor - segundo:.2f})")
            mapa = {str((r - i) % 10): (int(round(x)), int(round(y))) for i, (x, y) in enumerate(posicoes)}
            if len(self._cache_roda) > 32:
                self._cache_roda.clear()
            self._cache_roda[chave] = mapa
        return {d: (x + deslocamento[0], y + deslocamento[1]) for d, (x, y) in mapa.items()}

    def ponto_para_foco(self, frame):
        """Com o aviso "Clique no jogo" por cima do captcha: onde clicar para tirar o aviso.

        O aviso escurece a tela e cobre o teclado (run de 29/10: "0 botões soltos" sem fim).
        O ponto é a barra do título do quadro: fora do teclado e dos botões, então o clique
        não digita nada nem se o aviso sumir antes. None se não houver aviso ou quadro.
        """
        if frame is None or not isinstance(frame, np.ndarray) or frame.ndim != 3:
            return None
        if vision.achar(AVISO_FOCO, frame, config.LIMIAR) is None:
            return None
        pos = self.ancora(frame)
        if pos is None:
            return None
        ax, ay = self.perfil["anchor"]
        rx, ry = self.perfil["rest"]
        return pos[0] - ax + rx, pos[1] - ay + ry

    def _teclado(self, frame, origem):
        kx, ky, kw, kh = self.perfil["keyboard"]
        return frame[origem[1] + ky:origem[1] + ky + kh, origem[0] + kx:origem[0] + kx + kw]

    def conferir(self, frame, origem, digito, ponto):
        """Com o mouse parado em `ponto`: há um botão ali? Devolve (ok, centro do botão).

        Qual número é cada botão vem da rotação da roda (10 leituras juntas); um botão
        sozinho é mal classificado (no print real, o 9 parecia mais 8 e 0). Aqui só se
        corrige a mira para o centro do botão sob o mouse; sem botão perto, não clica.
        """
        kx, ky, _, _ = self.perfil["keyboard"]
        roda = self.perfil["ring"]
        tec = self._teclado(frame, origem)
        x, y = ponto[0] - origem[0] - kx, ponto[1] - origem[1] - ky
        hsv = cv2.cvtColor(tec, cv2.COLOR_BGR2HSV)
        mascara = (hsv[..., 2] < roda["dark_max"]).astype(np.uint8)
        mascara = cv2.morphologyEx(mascara, cv2.MORPH_CLOSE, np.ones((5, 5), np.uint8))
        n, rotulos, estat, centros = cv2.connectedComponentsWithStats(mascara)
        melhor, dist = None, roda["hover_raio"]
        for i in range(1, n):
            if not (roda["area_min"] <= estat[i, cv2.CC_STAT_AREA] <= roda["area_max"] * 2):
                continue
            esq, cima, larg, alt = estat[i, :4]
            # Distância do mouse até a caixa do botão (0 se estiver dentro dela).
            d = np.hypot(max(esq - x, 0, x - (esq + larg)), max(cima - y, 0, y - (cima + alt)))
            if d <= dist:
                melhor, dist = centros[i], d
        if melhor is None:
            return False, ponto
        return True, (int(round(melhor[0])) + origem[0] + kx, int(round(melhor[1])) + origem[1] + ky)

    def fase(self, frame, origem):
        """Fase da roda num print novo (rápido: não lê os números). None se não der."""
        try:
            return self.fase_da_roda(self._teclado(frame, origem))
        except QuadroInvalido:
            return None

    @staticmethod
    def girar(centro, ponto, graus):
        """Gira `ponto` em volta de `centro` (sentido horário na tela para graus > 0)."""
        a = np.radians(graus)
        dx, dy = ponto[0] - centro[0], ponto[1] - centro[1]
        return (int(round(centro[0] + dx * np.cos(a) - dy * np.sin(a))),
                int(round(centro[1] + dx * np.sin(a) + dy * np.cos(a))))

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
        try:
            teclas = {digito: self._alvo(teclado, digito, (ox+kx, oy+ky))
                      for digito in "0123456789"}
        except QuadroInvalido:
            # Os botões giram numa roda e mudam de tamanho a cada desafio: o recorte de
            # tamanho fixo só casa quando cada número cai no mesmo lugar da referência.
            teclas = self.teclas_pela_roda(teclado, (ox+kx, oy+ky))
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
        cx, cy = self.perfil["ring"]["center"]
        try:
            fase = self.fase_da_roda(teclado)
        except QuadroInvalido:
            fase = None
        return Quadro((ox, oy), codigo, self._recortar(popup, self.perfil["timer"]).copy(),
                      identidade, teclas, confirmar, resetar, ocupados,
                      (ox + kx + cx, oy + ky + cy), fase,
                      (ox + self.perfil["rest"][0], oy + self.perfil["rest"][1]))
