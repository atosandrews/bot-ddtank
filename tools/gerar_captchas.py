"""Gerador determinístico de recortes sintéticos do "Código de verificação" (164x68, BGR).

Imita a imagem real docs/especificacao/imagens/38_codigo_verificacao.png (código 6723):
fundo quase branco (~243), quatro dígitos grandes (40-65 px de altura) em cores
saturadas escuras, vizinhos às vezes sobrepostos ou com a mesma cor, linhas finas
claras e pouco saturadas e pontinhos coloridos. Só serve para medir o OCR offline;
acerto aqui não comprova acerto no jogo.

Uso:
    python tools/gerar_captchas.py --nivel medio --n 20 --semente 1 --saida logs/captchas_amostra

Não captura tela nem importa drivers.
"""
from __future__ import annotations

import argparse
import colorsys
from dataclasses import dataclass, field
from functools import lru_cache
from pathlib import Path

import cv2
import numpy as np

LARGURA, ALTURA = 164, 68
NIVEIS = ("facil", "medio", "dificil", "adversarial")
CASOS_ADVERSARIAIS = ("mesma_cor_separada", "colados", "palidas", "mesma_cor_toda")

# Fontes TrueType do Windows (as ausentes são ignoradas). Variedade de serifa, peso e itálico.
FONTES_TTF = (
    "arial.ttf", "ariali.ttf", "georgia.ttf", "georgiai.ttf", "times.ttf", "timesi.ttf",
    "timesbd.ttf", "comic.ttf", "verdana.ttf", "tahoma.ttf", "trebuc.ttf", "calibri.ttf",
    "cambriai.ttf", "couri.ttf",
)
# Traço fino (como o 6 roxo do real): fontes leves, sem engrossar.
FONTES_TTF_FINAS = ("calibril.ttf", "times.ttf", "timesi.ttf", "couri.ttf", "georgiai.ttf")
PASTA_FONTES = Path("C:/Windows/Fonts")
FONTES_HERSHEY = (
    cv2.FONT_HERSHEY_SIMPLEX, cv2.FONT_HERSHEY_DUPLEX, cv2.FONT_HERSHEY_COMPLEX,
    cv2.FONT_HERSHEY_TRIPLEX,  # SCRIPT fica fora: o "1" vira um "^" ilegível
)


@dataclass(frozen=True)
class Parametros:
    altura: tuple            # altura alvo do dígito (px)
    rotacao: float           # graus, +-
    inclinacao: float        # shear horizontal, +-
    sobreposicao: tuple      # fração da largura do dígito anterior (negativo = espaço)
    linhas: tuple            # quantidade de linhas de ruído
    linhas_por_cima: float   # fração das linhas desenhadas sobre os dígitos
    pontos: tuple            # quantidade de pontinhos
    pontos_escuros: float    # fração de pontos saturados/escuros (passam no filtro de dígito)
    saturacao: tuple         # S (0-1) da cor do dígito
    brilho: tuple            # V (0-1) da cor do dígito
    n_cores: tuple           # cores distintas possíveis entre os 4 dígitos
    traco_fino: float        # probabilidade de um dígito ter traço fino (como o 6 real)
    blur: float              # probabilidade de blur gaussiano leve
    jpeg: float              # probabilidade de recompressão JPEG


PARAMETROS = {
    "facil": Parametros((40, 52), 8, 0.08, (-0.25, -0.05), (3, 6), 0.2, (80, 150), 0.0,
                        (0.55, 0.85), (0.38, 0.58), (4, 4), 0.0, 0.3, 0.1),
    # "medio" é o mais parecido com a imagem 38: vizinhos encostando, às vezes mesma cor.
    "medio": Parametros((44, 60), 18, 0.2, (-0.08, 0.25), (10, 16), 0.35, (260, 380), 0.04,
                        (0.5, 0.85), (0.35, 0.6), (2, 4), 0.35, 0.6, 0.3),
    "dificil": Parametros((46, 64), 28, 0.3, (0.05, 0.3), (14, 22), 0.5, (340, 480), 0.08,
                          (0.45, 0.85), (0.33, 0.65), (2, 3), 0.4, 0.7, 0.5),
}


@dataclass
class Captcha:
    imagem: np.ndarray       # BGR uint8, 68x164x3
    rotulo: str              # 4 dígitos
    nivel: str
    caso: str = ""
    cores: list = field(default_factory=list)  # índice de cor por dígito (mesmo índice = mesma cor)


@lru_cache(maxsize=None)
def _fontes_ttf():
    try:
        from PIL import ImageFont  # noqa: F401
    except ImportError:
        return ()
    return tuple(str(PASTA_FONTES / f) for f in FONTES_TTF if (PASTA_FONTES / f).is_file())


@lru_cache(maxsize=256)
def _fonte_pil(caminho, tamanho):
    from PIL import ImageFont
    return ImageFont.truetype(caminho, tamanho)


def _recortar_mascara(m):
    ys, xs = np.nonzero(m > 0.05)
    if ys.size == 0:
        return m[:1, :1]
    return m[ys.min():ys.max() + 1, xs.min():xs.max() + 1]


def _glifo(rng, digito, fino):
    """Máscara float [0,1] do dígito, sem transformação, com bastante margem."""
    lado = 160
    fontes = _fontes_ttf()
    if fontes and rng.random() < 0.7:
        from PIL import Image, ImageDraw
        if fino:
            finas = [f for f in fontes if Path(f).name in FONTES_TTF_FINAS] or list(fontes)
            caminho = finas[int(rng.integers(len(finas)))]
        else:
            caminho = fontes[int(rng.integers(len(fontes)))]
        img = Image.new("L", (lado, lado), 0)
        d = ImageDraw.Draw(img)
        traco = 0 if fino else int(rng.integers(0, 2))
        d.text((lado // 2, lado // 2), digito, fill=255, font=_fonte_pil(caminho, 90),
               anchor="mm", stroke_width=traco, stroke_fill=255)
        return np.asarray(img, np.float32) / 255
    fonte = FONTES_HERSHEY[int(rng.integers(len(FONTES_HERSHEY)))]
    espessura = int(rng.integers(2, 4)) if fino else int(rng.integers(4, 8))
    m = np.zeros((lado, lado), np.uint8)
    escala = 3.0
    (w, h), _ = cv2.getTextSize(digito, fonte, escala, espessura)
    cv2.putText(m, digito, ((lado - w) // 2, (lado + h) // 2), fonte, escala, 255,
                espessura, cv2.LINE_AA)
    return m.astype(np.float32) / 255


def _transformar(rng, mascara, altura, p):
    m = _recortar_mascara(mascara)
    escala = altura / max(m.shape[0], 1)
    sx = escala * rng.uniform(0.8, 1.15)  # larguras variadas (condensado/expandido)
    m = cv2.resize(m, (max(1, int(m.shape[1] * sx)), max(1, int(m.shape[0] * escala))),
                   interpolation=cv2.INTER_AREA)
    pad = max(m.shape) // 2 + 4
    m = cv2.copyMakeBorder(m, pad, pad, pad, pad, cv2.BORDER_CONSTANT, value=0)
    h, w = m.shape
    ang = rng.uniform(-p.rotacao, p.rotacao)
    rot = cv2.getRotationMatrix2D((w / 2, h / 2), ang, 1.0)
    rot = np.vstack([rot, [0, 0, 1]])
    cis = np.array([[1, rng.uniform(-p.inclinacao, p.inclinacao), 0], [0, 1, 0], [0, 0, 1]])
    cis[0, 2] = -cis[0, 1] * h / 2
    m = cv2.warpAffine(m, (cis @ rot)[:2], (w, h), flags=cv2.INTER_LINEAR)
    return _recortar_mascara(m)


def _cor_hsv(h, s, v):
    r, g, b = colorsys.hsv_to_rgb(h % 1.0, s, v)
    return np.array([b * 255, g * 255, r * 255], np.float32)


def _paleta(rng, n, p):
    """n matizes bem separados (>= ~50 graus), como vermelho/roxo/azul/verde do jogo."""
    base = rng.random()
    passo = 1.0 / n
    matizes = [(base + i * passo + rng.uniform(-0.25, 0.25) * passo) for i in range(n)]
    rng.shuffle(matizes)
    return [_cor_hsv(hh, rng.uniform(*p.saturacao), rng.uniform(*p.brilho)) for hh in matizes]


def _atribuir_cores(rng, n_cores):
    """Índice de cor por dígito, usando exatamente n_cores cores (repetições em qualquer posição)."""
    while True:
        idx = list(range(n_cores)) + list(rng.integers(0, n_cores, 4 - n_cores))
        rng.shuffle(idx)
        if len(set(idx)) == n_cores:
            return [int(i) for i in idx]


def _fundo(rng):
    base = np.array([243, 243, 243], np.float32) + rng.uniform(-6, 4, 3)
    img = np.ones((ALTURA, LARGURA, 3), np.float32) * base
    # manchas leves de tom, como o leve bege do real
    ruido = cv2.GaussianBlur(rng.normal(0, 6, (ALTURA, LARGURA, 3)).astype(np.float32), (0, 0), 6)
    return np.clip(img + ruido, 0, 255)


def _desenhar_linhas(rng, img, quantidade):
    for _ in range(quantidade):
        # cinza-azulado, lilás, esverdeado: claro e pouco saturado (S < 0.3)
        cor = _cor_hsv(rng.random(), rng.uniform(0.0, 0.28), rng.uniform(0.45, 0.82))
        camada = np.zeros((ALTURA, LARGURA), np.uint8)
        n = int(rng.integers(2, 5))
        xs = np.sort(rng.uniform(-20, LARGURA + 20, n))
        ys = rng.uniform(-10, ALTURA + 10, n)
        if rng.random() < 0.5:
            xs, ys = rng.uniform(-10, LARGURA + 10, n), np.sort(rng.uniform(-10, ALTURA + 10, n))
        t = np.linspace(0, 1, 80)
        cx = np.interp(t, np.linspace(0, 1, n), xs)
        cy = np.interp(t, np.linspace(0, 1, n), ys)
        if rng.random() < 0.35:  # arco ondulado cruzando os dígitos, como no real
            cy = cy + rng.uniform(4, 12) * np.sin(2 * np.pi * rng.uniform(0.5, 1.5) * t + rng.uniform(0, 6.3))
        pts = np.stack([cx, cy], 1).astype(np.int32)
        # o real tem algumas faixas claras de 2-3 px além das linhas finas
        espessura = int(rng.choice([1, 1, 1, 1, 1, 2, 2, 3]))
        cv2.polylines(camada, [pts], False, 255, espessura, cv2.LINE_AA)
        opacidade = rng.uniform(0.7, 1.0) if espessura == 1 else rng.uniform(0.35, 0.6)
        a = camada.astype(np.float32)[..., None] / 255 * opacidade
        img[:] = img * (1 - a) + cor * a


def _desenhar_pontos(rng, img, quantidade, frac_escuros):
    for _ in range(quantidade):
        if rng.random() < frac_escuros:
            cor = _cor_hsv(rng.random(), rng.uniform(0.4, 0.8), rng.uniform(0.4, 0.75))
        else:
            cor = _cor_hsv(rng.random(), rng.uniform(0.1, 0.45), rng.uniform(0.5, 0.85))
        x, y = int(rng.integers(0, LARGURA)), int(rng.integers(0, ALTURA))
        lado = int(rng.choice([1, 1, 2, 2, 3]))
        img[y:y + lado, x:x + lado] = img[y:y + lado, x:x + lado] * 0.25 + cor * 0.75


def _sorteio(rng, faixa):
    a, b = faixa
    return int(rng.integers(a, b + 1)) if isinstance(a, int) else float(rng.uniform(a, b))


def gerar(semente: int, nivel: str = "medio", indice: int = 0, rotulo: str | None = None,
          caso: str | None = None) -> Captcha:
    """Gera um captcha. Mesmos (semente, nivel, indice, rotulo, caso) => mesma imagem."""
    if nivel not in NIVEIS:
        raise ValueError(f"nível desconhecido: {nivel}")
    rng = np.random.default_rng([int(semente), NIVEIS.index(nivel), int(indice)])
    if rotulo is None:
        rotulo = "".join(str(int(d)) for d in rng.integers(0, 10, 4))
    if not (len(rotulo) == 4 and rotulo.isdigit()):
        raise ValueError(f"rótulo inválido: {rotulo!r}")

    if nivel == "adversarial":
        caso = caso or CASOS_ADVERSARIAIS[int(indice) % len(CASOS_ADVERSARIAIS)]
        base = PARAMETROS["medio"]
        if caso == "mesma_cor_separada":
            # d1 e d3 (ou d2 e d4) da mesma cor com outra cor entre eles
            p, cores = base, ([0, 1, 0, 2] if rng.random() < 0.5 else [0, 1, 2, 1])
        elif caso == "colados":
            # vizinhos da mesma cor bem sobrepostos: a camada de cor junta dois dígitos
            p = Parametros(**{**base.__dict__, "sobreposicao": (0.2, 0.3)})
            cores = [0, 0, 1, 2] if rng.random() < 0.5 else [0, 1, 1, 2]
        elif caso == "palidas":
            # cores perto dos limiares da variante "cores" (S>=80/255, V<=200/255)
            p = Parametros(**{**base.__dict__, "saturacao": (0.28, 0.42), "brilho": (0.66, 0.82)})
            cores = None
        elif caso == "mesma_cor_toda":
            p, cores = base, [0, 0, 0, 0]
        else:
            raise ValueError(f"caso adversarial desconhecido: {caso}")
    else:
        p, cores, caso = PARAMETROS[nivel], None, caso or ""

    if cores is None:
        cores = _atribuir_cores(rng, _sorteio(rng, p.n_cores))
    paleta = _paleta(rng, max(cores) + 1, p)

    img = _fundo(rng)
    n_linhas = _sorteio(rng, p.linhas)
    por_cima = int(round(n_linhas * p.linhas_por_cima))
    _desenhar_linhas(rng, img, n_linhas - por_cima)
    _desenhar_pontos(rng, img, _sorteio(rng, p.pontos) // 2, p.pontos_escuros)

    glifos = []
    for d in rotulo:
        fino = rng.random() < p.traco_fino
        glifos.append(_transformar(rng, _glifo(rng, d, fino), _sorteio(rng, p.altura), p))
    # posições horizontais sequenciais: ordem esquerda->direita igual ao rótulo
    xs = [0.0]
    for anterior, proximo in zip(glifos, glifos[1:]):
        # sobreposição relativa ao menor dos dois: um dígito estreito nunca some dentro do outro
        menor = min(anterior.shape[1], proximo.shape[1])
        xs.append(xs[-1] + anterior.shape[1] - menor * _sorteio(rng, p.sobreposicao))
    largura_total = xs[-1] + glifos[-1].shape[1]
    if largura_total > LARGURA - 4:  # comprime para caber
        fator = (LARGURA - 4) / largura_total
        glifos = [cv2.resize(g, (max(1, int(g.shape[1] * fator)), max(1, int(g.shape[0] * fator))),
                             interpolation=cv2.INTER_AREA) for g in glifos]
        xs = [x * fator for x in xs]
        largura_total = (LARGURA - 4)
    x0 = rng.uniform(2, max(2.0, LARGURA - 2 - largura_total))
    ordem = rng.permutation(4)  # quem fica por cima na sobreposição
    for i in ordem:
        g = glifos[i]
        gh, gw = g.shape
        x = int(round(x0 + xs[i]))
        y = int(round(rng.uniform(-0.06 * gh, max(0.0, ALTURA - gh * 0.94))))
        ya, yb = max(0, y), min(ALTURA, y + gh)
        xa, xb = max(0, x), min(LARGURA, x + gw)
        if ya >= yb or xa >= xb:
            continue
        a = g[ya - y:yb - y, xa - x:xb - x][..., None]
        regiao = img[ya:yb, xa:xb]
        img[ya:yb, xa:xb] = regiao * (1 - a) + paleta[cores[i]] * a

    _desenhar_linhas(rng, img, por_cima)
    _desenhar_pontos(rng, img, _sorteio(rng, p.pontos) // 2, p.pontos_escuros)

    saida = np.clip(img, 0, 255).astype(np.uint8)
    if rng.random() < 0.5:  # o real parece reamostrado (bordas suaves, pontos em blocos)
        f = rng.uniform(0.6, 0.85)
        menor = cv2.resize(saida, (int(LARGURA * f), int(ALTURA * f)), interpolation=cv2.INTER_AREA)
        saida = cv2.resize(menor, (LARGURA, ALTURA), interpolation=cv2.INTER_LINEAR)
    if rng.random() < p.blur:
        saida = cv2.GaussianBlur(saida, (0, 0), rng.uniform(0.3, 0.6))
    if rng.random() < p.jpeg:
        ok, buf = cv2.imencode(".jpg", saida, [cv2.IMWRITE_JPEG_QUALITY, int(rng.integers(70, 95))])
        if ok:
            saida = cv2.imdecode(buf, cv2.IMREAD_COLOR)
    return Captcha(saida, rotulo, nivel, caso, cores)


def gerar_lote(semente: int, nivel: str, n: int):
    return [gerar(semente, nivel, i) for i in range(n)]


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--nivel", choices=NIVEIS + ("todos",), default="todos")
    ap.add_argument("--n", type=int, default=12)
    ap.add_argument("--semente", type=int, default=1)
    ap.add_argument("--saida", default="logs/captchas_amostra")
    ap.add_argument("--ampliar", type=int, default=1, help="salva também cópia ampliada (vizinho)")
    args = ap.parse_args(argv)
    pasta = Path(args.saida)
    pasta.mkdir(parents=True, exist_ok=True)
    niveis = NIVEIS if args.nivel == "todos" else (args.nivel,)
    for nivel in niveis:
        for i in range(args.n):
            c = gerar(args.semente, nivel, i)
            nome = f"{nivel}_{i:03d}_{c.rotulo}{'_' + c.caso if c.caso else ''}.png"
            img = c.imagem
            if args.ampliar > 1:
                img = cv2.resize(img, None, fx=args.ampliar, fy=args.ampliar,
                                 interpolation=cv2.INTER_NEAREST)
            cv2.imwrite(str(pasta / nome), img)
    print(f"{len(niveis) * args.n} imagens em {pasta}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
