"""Pintinho Corre Rápido: o que o bot lê na tela (só visão, sem clicar).

Templates recortados da gravação de 30/10 (logs/gravacao/20261030_100744, cliente 1920x1032,
DPI 100%), em img/pintinho/. Na batalha tudo é medido a partir do rótulo "Força" (âncora de
src/batalha.py), como no resto do bot; as regiões abaixo são deslocamentos (x1, y1, x2, y2).
"""
import cv2
import numpy as np

from . import vision
from .telas import IMG

PASTA = IMG / "pintinho"

# Quadro de missão (canto de cima à direita): a 2ª linha diz o estágio.
MISSAO = (660, -410, 780, -370)
ESTAGIOS = {
    1: PASTA / "missao_pintinhos.png",   # "Derrote os pintinhos"
    2: PASTA / "missao_galinhas.png",    # "Derrote 3 Galinhas"
    3: PASTA / "missao_grange.png",      # "Destrua o Grange"
    4: PASTA / "missao_gaiola.png",      # "Destrua a Gaiola"
}
LIMIAR_ESTAGIO = 0.75
# Contador do estágio 2 ("0/3", "1/3", "2/3"): quantas galinhas já morreram.
CONTADOR = (770, -410, 840, -370)
CONTADORES = {n: PASTA / f"contador_{n}de3.png" for n in range(3)}
LIMIAR_CONTADOR = 0.75

# Indicador "Força Vento" (seta branca com dois dígitos coloridos, ex.: ← 2•5).
VENTO = (320, -572, 418, -525)
PASTA_VENTO = PASTA / "vento"
LIMIAR_DIGITO_VENTO = 0.7
TAMANHO_DIGITO = (14, 20)  # moldura (largura, altura) em que os dígitos são comparados

# Minimapa (canto de cima à direita): rótulo "difícil" em cima; eu = ponto azul, inimigos = vermelhos.
ROTULO_MINIMAPA = PASTA / "minimapa_dificil.png"
BUSCA_ROTULO = (600, -600, 860, -540)
MINIMAPA = (-8, 8, 185, 110)  # em relação ao canto de cima à esquerda do rótulo
TOPO_MINIMAPA = 14  # acima disso (no recorte) é a faixa do contador e do botão de sair
# A área da câmera (moldura cinza) mede ~86,5 px no minimapa para os ~1000 px da tela, e a
# régua do jogo põe ~100 px em cada 1 de distância das tabelas (src/batalha.py).
DISTANCIA_POR_PX_MINIMAPA = 1000 / 86.5 / 100

# Retrato e barra de vida do chefe, no alto à direita do vento.
BARRA_CHEFE = (430, -565, 645, -525)
CHEFE_VERMELHO_MIN = 20

# Popup "Requisitos/Vitória" no começo de cada luta; o X fica à direita do título.
POPUP_MISSAO = PASTA / "missao_popup_titulo.png"
X_DO_POPUP = (323, -1)

# Telas de cartas no fim de cada estágio ("fim do jogo"): 9 cartas, e 21 no fim da instância.
CARTAS_FIM = [PASTA / "cartas_fim_do_jogo.png", PASTA / "cartas_fim_do_jogo_final.png"]
CARTA_VERSO = [PASTA / "carta_verso.png", PASTA / "carta_verso_final.png"]
LIMIAR_CARTAS = 0.75
LIMIAR_VERSO = 0.8


def _regiao(frame, ancora, caixa):
    """Recorte de `frame` numa caixa relativa à âncora; devolve (recorte, origem x, origem y)."""
    ax, ay = ancora
    x1, y1, x2, y2 = caixa
    ox, oy = max(ax + x1, 0), max(ay + y1, 0)
    return frame[oy:max(ay + y2, 0), ox:max(ax + x2, 0)], ox, oy


def amarelo(img):
    """Texto amarelo do quadro de missão (o fundo semitransparente muda com o mapa)."""
    b, g, r = (img[:, :, k].astype(int) for k in range(3))
    return ((r > 190) & (g > 170) & (b < 140)).astype(np.uint8) * 255


def _nota_amarelo(template, recorte):
    tpl = amarelo(vision.carregar(template))
    alvo = amarelo(recorte)
    if tpl.shape[0] > alvo.shape[0] or tpl.shape[1] > alvo.shape[1] or not alvo.any():
        return 0.0
    return float(cv2.matchTemplate(alvo, tpl, cv2.TM_CCOEFF_NORMED).max())


def _melhor(templates, recorte, limiar):
    notas = {chave: _nota_amarelo(t, recorte) for chave, t in templates.items()}
    chave = max(notas, key=notas.get)
    return chave if notas[chave] >= limiar else None


def ler_estagio(frame, ancora):
    """1 a 4 pelo texto do quadro de missão, ou None (quadro coberto ou fora da batalha)."""
    recorte, _, _ = _regiao(frame, ancora, MISSAO)
    return _melhor(ESTAGIOS, recorte, LIMIAR_ESTAGIO)


def galinhas_mortas(frame, ancora):
    """Estágio 2: 0, 1 ou 2 galinhas mortas (contador "n/3"), ou None se não der para ler."""
    recorte, _, _ = _regiao(frame, ancora, CONTADOR)
    return _melhor(CONTADORES, recorte, LIMIAR_CONTADOR)


def _mascaras_vento(recorte):
    hsv = cv2.cvtColor(recorte, cv2.COLOR_BGR2HSV).astype(int)
    branco = (hsv[:, :, 1] < 50) & (hsv[:, :, 2] > 190)
    cor = (hsv[:, :, 1] > 45) & (hsv[:, :, 2] > 100)
    return branco, cor


def _digitos_vento(cor):
    """Máscaras dos dois dígitos, da esquerda para a direita (ou None).

    Os dígitos são as manchas coloridas dentro da seta (altura >= 13); o ponto entre eles é
    menor. Pedaços do mesmo dígito (o 7 às vezes vem em 2) são juntados pela coluna.
    """
    k, rotulos, stats, _ = cv2.connectedComponentsWithStats(cor.astype(np.uint8))
    pedacos = []
    for i in range(1, k):
        x, y, w, h, area = stats[i]
        if x >= 6 and x + w <= cor.shape[1] - 12 and y >= 10 and area < 400:
            pedacos.append((x, y, w, h, i))
    # Os dígitos começam na mesma faixa (y 10–18); embaixo da seta há sujeira do cenário.
    grandes = sorted((p for p in pedacos if 13 <= p[3] <= 20 and p[2] >= 5 and p[1] <= 18),
                     key=lambda p: p[0])
    if len(grandes) != 2:
        return None
    digitos = []
    for x, y, w, h, i in grandes:
        mascara = rotulos == i
        for px, py, pw, ph, j in pedacos:  # pedaços soltos do mesmo dígito (topo do 7 e do 3)
            if (j != i and px >= x - 2 and px + pw <= x + w + 2 and ph < 13
                    and py >= y - 6 and py + ph <= y + h + 1):
                mascara |= rotulos == j
        ys, xs = np.where(mascara)
        digitos.append(mascara[ys.min():ys.max() + 1, xs.min():xs.max() + 1].astype(np.uint8) * 255)
    return digitos


def _normalizar(mascara):
    """Centraliza o dígito numa moldura fixa sem esticar (o 1 é estreito; esticado vira um bloco)."""
    largura, altura = TAMANHO_DIGITO
    h, w = mascara.shape
    if h > altura or w > largura:
        escala = min(altura / h, largura / w)
        mascara = cv2.resize(mascara, (max(int(w * escala), 1), max(int(h * escala), 1)),
                             interpolation=cv2.INTER_AREA)
        h, w = mascara.shape
    moldura = np.zeros((altura, largura), float)
    y, x = (altura - h) // 2, (largura - w) // 2
    moldura[y:y + h, x:x + w] = mascara / 255
    return moldura


_modelos_vento = {}


def _carregar_modelos_vento():
    if not _modelos_vento and PASTA_VENTO.exists():
        for arq in sorted(PASTA_VENTO.glob("*.png")):
            img = cv2.imread(str(arq), cv2.IMREAD_GRAYSCALE)
            _modelos_vento.setdefault(arq.stem[0], []).append(_normalizar(img))
    return _modelos_vento


def _classificar(mascara):
    """O dígito mais parecido (pelo formato) ou None se nenhum passar do limiar."""
    alvo = _normalizar(mascara)
    melhor, nota = None, 0.0
    for digito, modelos in _carregar_modelos_vento().items():
        for modelo in modelos:
            inter = np.minimum(alvo, modelo).sum()
            uniao = np.maximum(alvo, modelo).sum()
            n = inter / uniao if uniao else 0.0
            if n > nota:
                melhor, nota = digito, n
    return melhor if nota >= LIMIAR_DIGITO_VENTO else None


def lado_do_vento(branco):
    """-1 = seta para a esquerda, 1 = para a direita, pela ponta (a parte mais larga da seta)."""
    k, rotulos, stats, _ = cv2.connectedComponentsWithStats(branco.astype(np.uint8))
    if k < 2:
        return None
    seta = 1 + int(np.argmax(stats[1:, cv2.CC_STAT_AREA]))
    if stats[seta, cv2.CC_STAT_AREA] < 300:
        return None
    m = rotulos == seta
    cols = np.where(m.any(axis=0))[0]
    esq = m[:, cols.min():cols.min() + 15].sum()
    dir_ = m[:, cols.max() - 14:cols.max() + 1].sum()
    return -1 if esq > 1.6 * dir_ else 1


def ler_vento(frame, ancora):
    """Vento com sinal (positivo = soprando para a direita), ex.: -2.5. None se ilegível."""
    recorte, _, _ = _regiao(frame, ancora, VENTO)
    if recorte.size == 0:
        return None
    branco, cor = _mascaras_vento(recorte)
    lado = lado_do_vento(branco)
    digitos = _digitos_vento(cor)
    if lado is None or digitos is None:
        return None
    lidos = [_classificar(d) for d in digitos]
    if None in lidos:
        return None
    return lado * (int(lidos[0]) + int(lidos[1]) / 10)


def pontos_minimapa(frame, ancora):
    """Pontos do minimapa em pixels da tela: (x meu ou None, [(x, y) vermelhos], [(x, y) verdes]).

    Vermelho = inimigo; verde-amarelo = a gaiola do estágio 4. None sem o minimapa.
    A faixa de cima (contador e botão vermelho de sair) fica de fora.
    """
    busca, ox, oy = _regiao(frame, ancora, BUSCA_ROTULO)
    pos, nota = vision.procurar(ROTULO_MINIMAPA, busca) if busca.size else (None, 0)
    if pos is None or nota < 0.8:
        return None
    h, w = vision.carregar(ROTULO_MINIMAPA).shape[:2]
    canto = (ox + pos[0] - w // 2, oy + pos[1] - h // 2)
    mapa, mx, my = _regiao(frame, canto, MINIMAPA)
    b, g, r = (mapa[:, :, k].astype(int) for k in range(3))
    azul = (b > 180) & (r < 80) & (g < 120)
    eu = None
    if azul.sum() >= 6:
        eu = int(np.median(np.where(azul)[1])) + mx

    def pontos(mascara):
        k, _, stats, centros = cv2.connectedComponentsWithStats(mascara.astype(np.uint8))
        return sorted((int(round(centros[i][0])) + mx, int(round(centros[i][1])) + my)
                      for i in range(1, k) if stats[i][4] >= 6 and centros[i][1] >= TOPO_MINIMAPA)

    vermelhos = pontos((r > 190) & (g < 80) & (b < 80))
    verdes = pontos((b < 30) & (abs(g - 204) < 14) & (abs(r - 153) < 14))  # ponto (153, 204, 0)
    return eu, vermelhos, verdes


def ler_minimapa(frame, ancora):
    """(x meu, [x dos inimigos]) no minimapa, ou None. Só o x interessa (lado e ordem)."""
    pontos = pontos_minimapa(frame, ancora)
    if pontos is None:
        return None
    eu, vermelhos, _ = pontos
    return eu, sorted(x for x, _ in vermelhos)


def lado_dos_inimigos(frame, ancora):
    """Pelo minimapa: 1 se os inimigos estão à direita de mim, -1 à esquerda, None sem saber."""
    mapa = ler_minimapa(frame, ancora)
    if mapa is None or mapa[0] is None or not mapa[1]:
        return None
    eu, inimigos = mapa
    direita = sum(1 for x in inimigos if x > eu)
    esquerda = len(inimigos) - direita
    if direita == esquerda:
        return None
    return 1 if direita > esquerda else -1


def chefe_vivo(frame, ancora):
    """Barra de vida do chefe no alto (ao lado do vento) ainda tem vermelho?

    No estágio 4 é o Burrinho: enquanto ele vive, a gaiola tem escudo. Na gravação a barra
    caiu de ~900 px vermelhos (100%) até ~130 (11%) e ficou em 0 com ele morto.
    """
    barra, _, _ = _regiao(frame, ancora, BARRA_CHEFE)
    b, g, r = (barra[:, :, k].astype(int) for k in range(3))
    return int(((r > 170) & (g < 90) & (b < 90)).sum()) >= CHEFE_VERMELHO_MIN


def x_do_popup_missao(frame):
    """Centro do X do popup "Requisitos/Vitória", ou None se ele não estiver na tela."""
    pos = vision.achar(POPUP_MISSAO, frame, 0.8)
    return None if pos is None else (pos[0] + X_DO_POPUP[0], pos[1] + X_DO_POPUP[1])


def tela_de_cartas(frame):
    """None fora da tela de cartas; "estagio" (9 cartas) ou "final" (21) dentro dela.

    O texto "fim do jogo" é igual nas duas; o que muda é o número de cartas (no máximo
    2 viram no fim, então lá sempre sobram mais de 12)."""
    if not any(vision.achar(tpl, frame, LIMIAR_CARTAS) for tpl in CARTAS_FIM):
        return None
    return "final" if len(cartas_fechadas(frame)) > 12 else "estagio"


def cartas_fechadas(frame):
    """Centros das cartas ainda viradas para baixo (verso com o baú), sem repetir."""
    achados = []
    for tpl in CARTA_VERSO:
        modelo = vision.carregar(tpl)
        h, w = modelo.shape[:2]
        res = cv2.matchTemplate(frame, modelo, cv2.TM_CCOEFF_NORMED)
        for y, x in sorted(zip(*np.where(res >= LIMIAR_VERSO)), key=lambda p: -res[p]):
            cx, cy = int(x) + w // 2, int(y) + h // 2
            if all(abs(cx - a) > w // 2 or abs(cy - b) > h // 2 for a, b in achados):
                achados.append((cx, cy))
    return achados
