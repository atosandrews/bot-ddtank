"""Adaptador do SDK local; não captura tela nem envia comandos ao jogo."""
from dataclasses import dataclass
from functools import lru_cache
import re

import cv2
import numpy as np

# Variante "cores": pixels de dígito são saturados e escuros; o ruído é claro.
SATURACAO_MIN_DIGITO = 80
BRILHO_MAX_DIGITO = 200
AREA_MIN_CAMADA = 40  # px no recorte de 164x68 do perfil nativo
CORES_POR_CAPTCHA = (2, 3, 4)


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

    @staticmethod
    def camadas_por_cor(imagem):
        """Separa os dígitos pela cor; cada agrupamento lista (x, pixels, xs_dos_pedacos) por camada.

        Dígitos sobrepostos de cores diferentes confundem o modelo quando lidos juntos.
        O ruído do captcha é claro e pouco saturado, então fica fora das camadas.
        """
        if imagem is None or imagem.size == 0 or imagem.ndim != 3:
            return []
        h, s, v = (c.astype(int) for c in cv2.split(cv2.cvtColor(imagem, cv2.COLOR_BGR2HSV)))
        mascara = (s >= SATURACAO_MIN_DIGITO) & (v <= BRILHO_MAX_DIGITO)
        ys, xs = np.nonzero(mascara)
        if ys.size < AREA_MIN_CAMADA:
            return []
        matizes = h[mascara]
        angulos = matizes * np.pi / 90  # matiz do OpenCV vai de 0 a 180 e é circular
        pontos = np.stack([np.cos(angulos), np.sin(angulos)], 1)
        # Só cores com massa de dígito podem iniciar um grupo; pixels isolados atrairiam centros.
        faixas = (matizes // 10) % 18
        densos = np.bincount(faixas, minlength=18)[faixas] >= AREA_MIN_CAMADA
        if not densos.any():
            return []
        agrupamentos = []
        for k in CORES_POR_CAPTCHA:
            grupos = _agrupar(pontos, k, densos)
            camadas = []
            for grupo in range(k):
                camada = np.full(mascara.shape, 255, np.uint8)
                camada[ys[grupos == grupo], xs[grupos == grupo]] = 0
                n, _, estat, centros = cv2.connectedComponentsWithStats((camada == 0).astype(np.uint8))
                if n < 2:
                    continue
                areas = estat[1:, cv2.CC_STAT_AREA]
                maior = 1 + int(np.argmax(areas))
                if areas.max() < AREA_MIN_CAMADA:
                    continue  # só pontos soltos de ruído
                pedacos = [float(c[0]) for c, a in zip(centros[1:], areas) if a >= AREA_MIN_CAMADA]
                camada = cv2.copyMakeBorder(camada, 6, 6, 10, 10, cv2.BORDER_CONSTANT, value=255)
                camadas.append((float(centros[maior][0]), camada, pedacos))
            if camadas:
                agrupamentos.append(sorted(camadas, key=lambda c: c[0]))
        return agrupamentos

    def _ler(self, nome, modelo, pixels):
        ok, png = cv2.imencode(".png", pixels)
        if not ok:
            return None
        try:
            resposta = modelo.classification(png.tobytes(), probability=True)
            return resposta.get("text", ""), float(resposta.get("confidence", 0.0))
        except Exception as exc:
            raise ErroInfraestrutura(f"Falha de inferência no modelo {nome}: {exc}") from exc

    def reconhecer(self, imagem, limiar=0.90):
        unicos = {}
        self.ultimas_leituras = []

        def registrar(texto, confianca, nome, variante):
            self.ultimas_leituras.append({"texto": texto, "confianca": confianca,
                                         "modelo": nome, "variante": variante})
            if (isinstance(texto, str) and re.fullmatch(r"[0-9]{4}", texto)
                    and np.isfinite(confianca) and limiar <= confianca <= 1.0):
                candidato = Candidato(texto, confianca, nome, variante)
                if texto not in unicos or confianca > unicos[texto].confianca:
                    unicos[texto] = candidato

        for variante, pixels in self.variantes(imagem):
            for nome, modelo in self.modelos:
                leitura = self._ler(nome, modelo, pixels)
                if leitura is not None:
                    registrar(*leitura, nome, variante)
        for camadas in self.camadas_por_cor(imagem):
            for nome, modelo in self.modelos:
                leituras = [self._ler(nome, modelo, pixels) for _, pixels, _ in camadas]
                # Uma camada sem leitura deixaria o código com dígitos faltando.
                if not all(l is not None and isinstance(l[0], str) and l[0] for l in leituras):
                    continue
                # Vários dígitos numa camada com outra camada no meio: não há ordem confiável.
                if any(len(l[0]) > 1 and any(min(pedacos) < x < max(pedacos)
                                             for j, (x, _, _) in enumerate(camadas) if j != i)
                       for i, (l, (_, _, pedacos)) in enumerate(zip(leituras, camadas))):
                    continue
                registrar("".join(l[0] for l in leituras), min(l[1] for l in leituras),
                          nome, "cores")
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


def _agrupar(pontos, k, densos, iteracoes=20):
    """k-means determinístico sobre matizes no círculo (sem semente aleatória)."""
    candidatos = pontos[densos]
    centros = [candidatos[int(np.argmax(candidatos @ pontos.mean(0)))]]
    while len(centros) < k:
        distancia = np.min([1 - candidatos @ c for c in centros], axis=0)
        centros.append(candidatos[int(np.argmax(distancia))])
    centros = np.array(centros)
    for _ in range(iteracoes):
        grupos = np.argmax(pontos @ centros.T, axis=1)
        novos = np.array([pontos[grupos == g].mean(0) if np.any(grupos == g) else centros[g]
                          for g in range(k)])
        if np.allclose(novos, centros):
            break
        centros = novos
    return grupos


@lru_cache(maxsize=1)
def obter_ocr():
    """Uma instância dos dois modelos por processo, compartilhada entre contas."""
    return OcrLocal()
