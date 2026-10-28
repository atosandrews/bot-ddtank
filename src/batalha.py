"""Batalha: lê a tela da luta e joga o turno conforme a estratégia do estágio.

Tudo é medido a partir do rótulo "Força" (canto inferior esquerdo), que fica sempre no
mesmo lugar do HUD. As posições abaixo são deslocamentos em pixels a partir do centro dele.
As estratégias de cada estágio ficam em config.ESTRATEGIAS.
"""
import time
from dataclasses import dataclass

import cv2
import numpy as np

from . import config, vision
from .telas import IMG

ANCORA = IMG / "batalha_forca.png"
PASS = IMG / "batalha_pass.png"
# O botão PASS é semitransparente: na caverna da Pandora ele bate só 0.80 (fora da vez, ~0.33).
LIMIAR_PASS = 0.65
# "Pandora" no quadro de missão ("Derrote Pandora"); o fundo do quadro é semitransparente,
# por isso o limiar é mais baixo (0.91+ no estágio 2, até 0.53 no estágio 1).
MISSAO_PANDORA = IMG / "missao_pandora.png"
LIMIAR_MISSAO = 0.75

# Barra de força: x do 0 e do 100, e a linha (y) onde medir.
BARRA_X0, BARRA_X100, BARRA_Y = 25, 522, -1
# Centro (pivô) da agulha do relógio de ângulo e o raio onde procurar a agulha.
AGULHA_PIVO = (-75, -36)  # calibrado com 6 prints de ângulo conhecido (erro médio ~1°)
AGULHA_RAIO = (10, 45)
# Região do número do relógio de ângulo (x1, y1, x2, y2) e os dígitos conhecidos.
# Os números mudam de cor, mas o formato é o mesmo: comparamos só o formato (máscara).
NUMERO = (-100, -34, -50, -6)
PASTA_DIGITOS = IMG / "digitos"
LIMIAR_DIGITO = 0.75
# Marcador vermelho do último tiro, logo acima da barra.
MARCADOR_Y = (-18, -13)
# Campo da luta onde procurar as barras de vida (sem o HUD de cima e a coluna de itens).
CAMPO = (-126, -449, 824, -29)  # x1, y1, x2, y2

# Força para cada distância (1 a 20), copiadas das tabelas da moldura do cliente.
TABELAS = {
    20: [10, 19, 25, 30, 36, 40, 44, 48, 51, 54, 57, 60, 63, 66, 69, 72, 74, 76, 78, 80],
    30: [14, 20, 24, 28, 32, 35, 38, 41, 44, 47, 50, 52, 55, 57, 60, 62, 65, 67, 69, 72],
    50: [14, 20, 24, 28, 32, 35, 39, 42, 44, 48, 50, 53, 55, 58, 60, 63, 65, 68, 70, 72],
    65: [13, 20, 26, 31, 37, 41, 44, 48, 53, 56, 58, 61, 64, 67, 70, 73, 76, 79, 82, 85],
}


def forca_para(angulo, distancia):
    """Força da tabela do ângulo, interpolando entre as distâncias inteiras (1 a 20)."""
    tabela = TABELAS[angulo]
    d = min(max(distancia, 1), len(tabela))
    i = int(d) - 1
    if i >= len(tabela) - 1:
        return float(tabela[-1])
    frac = d - int(d)
    return tabela[i] + (tabela[i + 1] - tabela[i]) * frac


# O triângulo azul da vez fica ~106 px acima da minha barra de vida.
TRIANGULO_ATE_BARRA = 106


def _mascara(hsv, cor):
    if cor == "vermelho":
        return cv2.inRange(hsv, (0, 150, 150), (8, 255, 255)) | cv2.inRange(hsv, (172, 150, 150), (180, 255, 255))
    return cv2.inRange(hsv, (100, 180, 150), (125, 255, 255))


def _barras(hsv, cor, largura_min=20):
    """Retorna os centros (x, y) das barras de vida da cor pedida ("vermelho" ou "azul")."""
    n, _, stats, _ = cv2.connectedComponentsWithStats(_mascara(hsv, cor))
    centros = []
    for x, y, w, h, area in stats[1:]:
        # Barra de vida: retângulo baixo, comprido e quase todo preenchido.
        if w >= largura_min and 2 <= h <= 8 and area >= 0.75 * w * h:
            centros.append((int(x + w // 2), int(y + h // 2)))
    return centros


def _triangulo(hsv):
    """Centro do triângulo azul que marca quem está jogando (ou None)."""
    n, _, stats, _ = cv2.connectedComponentsWithStats(_mascara(hsv, "azul"))
    for x, y, w, h, area in stats[1:]:
        # Triângulo: mais ou menos quadrado e só metade preenchido.
        if 10 <= w <= 30 and 8 <= h <= 24 and 0.35 * w * h <= area <= 0.75 * w * h:
            return int(x + w // 2), int(y + h // 2)
    return None


@dataclass
class Leitura:
    ancora: tuple
    minha_vez: bool
    eu: tuple | None
    inimigos: list
    estagio: str


def ler(frame):
    """Lê a tela da batalha. Retorna None se não estiver numa batalha."""
    ancora = vision.achar(ANCORA, frame, config.LIMIAR)
    if ancora is None:
        return None
    ax, ay = ancora
    x1, y1, x2, y2 = CAMPO
    ox, oy = max(ax + x1, 0), max(ay + y1, 0)
    campo = frame[oy:ay + y2, ox:ax + x2]
    hsv = cv2.cvtColor(campo, cv2.COLOR_BGR2HSV)
    # Uma formiga em cima de mim cobre parte da minha barra: aceita pedaços menores
    # e, se nem assim achar, usa o triângulo da vez.
    azuis = _barras(hsv, "azul", largura_min=10)
    triangulo = _triangulo(hsv)
    if azuis:
        eu = (azuis[0][0] + ox, azuis[0][1] + oy)
    elif triangulo:
        eu = (triangulo[0] + ox, triangulo[1] + oy + TRIANGULO_ATE_BARRA)
    else:
        eu = None
    inimigos = [(x + ox, y + oy) for x, y in _barras(hsv, "vermelho")]
    minha_vez = vision.achar(PASS, frame, LIMIAR_PASS) is not None
    estagio = "PANDORA" if vision.achar(MISSAO_PANDORA, frame, LIMIAR_MISSAO) else "FORMIGAS"
    return Leitura(ancora, minha_vez, eu, inimigos, estagio)


_digitos = {}


def _carregar_digitos():
    if not _digitos:
        # O nome do arquivo começa com o dígito; variações do mesmo dígito: "1_b.png", "1_c.png"...
        for arq in sorted(PASTA_DIGITOS.glob("*.png")):
            _digitos[arq.stem] = cv2.imread(str(arq), cv2.IMREAD_GRAYSCALE)
    return _digitos


def mascara_numero(frame, ancora):
    """Máscara (preto e branco) dos dígitos do relógio: colorido e claro = dígito."""
    ax, ay = ancora
    x1, y1, x2, y2 = NUMERO
    regiao = frame[ay + y1:ay + y2, max(ax + x1, 0):ax + x2]
    hsv = cv2.cvtColor(regiao, cv2.COLOR_BGR2HSV)
    return ((hsv[:, :, 1] > 90) & (hsv[:, :, 2] > 110)).astype(np.uint8) * 255


def ler_numero(frame, ancora):
    """Lê o número do relógio de ângulo. Retorna (valor, exato) ou (None, False).

    Só conhecemos os dígitos de img/digitos/ (0 a 5 por enquanto). Se aparecer um dígito
    desconhecido, o valor é aproximado: dezena conhecida + 5 (serve para saber a direção).
    """
    mascara = mascara_numero(frame, ancora)
    if mascara.sum() == 0:
        return None, False
    achados = []
    for nome, tpl in _carregar_digitos().items():
        if tpl.shape[0] > mascara.shape[0] or tpl.shape[1] > mascara.shape[1]:
            continue
        res = cv2.matchTemplate(mascara, tpl, cv2.TM_CCOEFF_NORMED)
        for y, x in zip(*np.where(res >= LIMIAR_DIGITO)):
            achados.append((float(res[y, x]), int(x), nome[0], tpl.shape[1]))
    # Fica com o melhor dígito em cada posição; um dígito não pode ficar por cima de outro.
    escolhidos = []
    for nota, x, digito, w in sorted(achados, reverse=True):
        if all(min(x + w, x2 + w2) - max(x, x2) <= 2 for _, x2, _, w2 in escolhidos):
            escolhidos.append((nota, x, digito, w))
    escolhidos.sort(key=lambda e: e[1])
    # Colunas com dígito que nenhum modelo explicou = dígito desconhecido.
    colunas = mascara.any(axis=0)
    for _, x, _, w in escolhidos:
        colunas[x:x + w] = False
    desconhecido = colunas.sum() > 2
    if not escolhidos:
        return None, False
    texto = "".join(d for _, _, d, _ in escolhidos)
    if not desconhecido:
        return int(texto), True
    largura = np.where(mascara.any(axis=0))[0]
    dois_digitos = largura.max() - largura.min() > 12
    if dois_digitos and len(texto) == 1 and escolhidos[0][1] <= largura.min() + 2:
        return int(texto) * 10 + 5, False  # dezena conhecida, unidade desconhecida
    return None, False


def ler_angulo(frame, ancora):
    """Ângulo atual (graus acima da horizontal) pela agulha vermelha do relógio, ou None.

    Só olha a metade de cima do relógio (os números ficam embaixo). A agulha aponta para o
    lado em que o personagem está virado; às vezes aparecem as duas, e aí usa a média.
    """
    px, py = ancora[0] + AGULHA_PIVO[0], ancora[1] + AGULHA_PIVO[1]
    r_min, r_max = AGULHA_RAIO
    oy, ox = max(py - r_max, 0), max(px - r_max, 0)
    regiao = frame[oy:py, ox:px + r_max + 1].astype(int)
    r, g, b = regiao[:, :, 2], regiao[:, :, 1], regiao[:, :, 0]
    vermelho = (r - np.maximum(g, b) > 70) & (r > 120)
    ys, xs = np.where(vermelho)
    dx, dy = xs + ox - px, py - (ys + oy)
    perto = (np.hypot(dx, dy) >= r_min) & (np.hypot(dx, dy) <= r_max)
    angulos = [float(np.median(np.degrees(np.arctan2(dy[lado], np.abs(dx[lado])))))
               for lado in (perto & (dx < 0), perto & (dx > 0)) if lado.sum() >= 5]
    return sum(angulos) / len(angulos) if angulos else None


def lado_virado(frame, ancora):
    """Para onde o personagem está virado (1 = direita, -1 = esquerda), pela agulha
    vermelha forte do relógio, que aponta para esse lado. None se não der para saber."""
    px, py = ancora[0] + AGULHA_PIVO[0], ancora[1] + AGULHA_PIVO[1]
    ox = max(px - AGULHA_RAIO[1], 0)
    regiao = frame[max(py - AGULHA_RAIO[1], 0):py, ox:px + AGULHA_RAIO[1] + 1].astype(int)
    forte = (regiao[:, :, 2] > 190) & (regiao[:, :, 1] < 70) & (regiao[:, :, 0] < 70)
    dx = np.where(forte)[1] + ox - px
    esquerda, direita = int((dx < -8).sum()), int((dx > 8).sum())
    if max(esquerda, direita) < 8:
        return None
    return 1 if direita > esquerda else -1


def forca_na_barra(referencia, atual):
    """Quanto da barra (0 a 100) mudou em relação à barra vazia de referência."""
    diff = np.abs(atual.astype(int) - referencia.astype(int)).mean(axis=(0, 2))
    mudou = diff > 40
    if not mudou[:4].any():
        return 0.0
    # A barra enche do 0 para a direita: conta só a faixa contínua desde o começo
    # (tolerando falhas de até 5 colunas), para uma animação passando por cima não virar 100.
    parado = np.convolve(~mudou, np.ones(6, dtype=int), "valid") == 6
    fim = int(np.argmax(parado)) if parado.any() else len(mudou)
    return fim / (referencia.shape[1] - 1) * 100


def marcador(frame, ancora):
    """Força do último tiro, pela posição do marcador vermelho acima da barra (ou None)."""
    ax, ay = ancora
    faixa = frame[ay + MARCADOR_Y[0]:ay + MARCADOR_Y[1], ax + BARRA_X0:ax + BARRA_X100 + 1].astype(int)
    vermelho = (faixa[:, :, 2] > 170) & (faixa[:, :, 1] < 90) & (faixa[:, :, 0] < 90)
    xs = np.where(vermelho.any(axis=0))[0]
    if len(xs) == 0:
        return None
    return float(xs.mean() / (BARRA_X100 - BARRA_X0) * 100)


class Batalha:
    def __init__(self, driver, log, salvar_debug):
        self.driver = driver
        self.log = log
        self.salvar_debug = salvar_debug
        self.turno = 0
        self.estagio = None
        self.virado = None  # lado para onde o personagem está virado (1, -1 ou None)
        self.lado_alternado = 1  # 1 = direita, -1 = esquerda

    def passo(self, frame):
        leitura = ler(frame)
        if leitura is None or not leitura.minha_vez:
            time.sleep(0.5)
            return
        self.turno += 1
        if leitura.estagio != self.estagio:
            self.estagio = leitura.estagio
            self.virado = None  # estágio novo: não sei para onde o personagem está virado
            self.log(f"estágio: {self.estagio}")
        estrategia = config.ESTRATEGIAS[leitura.estagio]

        distancia, colado = None, False
        if estrategia["alvo"] in ("direita", "esquerda"):
            lado = 1 if estrategia["alvo"] == "direita" else -1
        elif leitura.eu and leitura.inimigos:
            ex = leitura.eu[0]
            alvo = min(leitura.inimigos, key=lambda p: abs(p[0] - ex))
            lado = 1 if alvo[0] > ex else -1
            distancia = abs(alvo[0] - ex) / config.PX_POR_DISTANCIA
            colado = abs(alvo[0] - ex) <= estrategia.get("colado_px", 0)
        elif leitura.eu and "forca_colado" in estrategia:
            # Eu apareço, mas nenhum inimigo: a barra dele está escondida atrás da minha,
            # ou seja, ele está exatamente em cima de mim. Alterna o lado a cada turno.
            self.log("nenhum inimigo à vista: deve estar colado em mim")
            self.salvar_debug(frame, "colado")
            self.lado_alternado = -self.lado_alternado
            lado, colado = self.lado_alternado, True
        elif estrategia["forca"] is not None:
            # Sem saber onde estão (ex.: formiga em cima de mim): alterna os lados a cada turno.
            self.log(f"não achei {'a mim' if leitura.eu is None else 'inimigos'}; atirando no outro lado")
            self.salvar_debug(frame, "sem_alvo")
            self.lado_alternado = -self.lado_alternado
            lado = self.lado_alternado
        else:
            self.log(f"minha vez, mas não achei {'a mim' if leitura.eu is None else 'inimigos'}; passando")
            self.salvar_debug(frame, "sem_alvo")
            pos = vision.achar(PASS, frame, LIMIAR_PASS)
            if pos:
                self.driver.clicar(*pos)
            self.esperar_fim_da_vez()
            return
        self.jogar(leitura, estrategia, lado, distancia, colado)

    def jogar(self, leitura, estrategia, lado, distancia, colado=False):
        angulo = estrategia["angulo"]
        if colado:
            forca = estrategia["forca_colado"]
        elif estrategia["forca"] is not None:
            forca = estrategia["forca"]
        else:
            forca = forca_para(angulo, distancia)
        onde = "colado" if colado else ("?" if distancia is None else f"{distancia:.1f}")
        self.log(f"turno {self.turno}: alvo a {onde} para a {'direita' if lado > 0 else 'esquerda'}, "
                 f"ângulo {angulo}, força {forca:.0f}")

        tecla_lado = config.TECLA_DIREITA if lado > 0 else config.TECLA_ESQUERDA
        # Uma batidinha na seta vira o personagem para o alvo. Só quando precisa: se ele já
        # está virado, a batidinha faz andar um pouco e pode mudar o ângulo.
        # A agulha do relógio mostra o lado; se não der para ver, usa o último lado virado.
        virado = lado_virado(self.driver.capturar(), leitura.ancora)
        if virado is not None:
            self.virado = virado
        if self.virado != lado:
            self.driver.tecla(tecla_lado, 0.05)
            self.virado = lado
            time.sleep(0.2)

        if config.AJUSTAR_ANGULO:
            andar = estrategia["andar"]
            for passo in range(config.MAX_PASSOS + 1):
                # Já no ângulo certo (ou chegou nele): não anda nem mexe em mais nada.
                if self.ajustar_angulo(leitura.ancora, angulo):
                    break
                if passo == 0:
                    self.salvar_debug(self.driver.capturar(), "angulo_travou")
                if andar is None or passo == config.MAX_PASSOS:
                    self.log("atirando com o ângulo que deu")
                    break
                # O chão aqui não deixa esse ângulo: anda um pouco e vira de volta para o alvo.
                self.log(f"ângulo {angulo} não dá aqui; andando para a {andar} (passo {passo + 1})")
                self.driver.tecla(config.TECLA_ESQUERDA if andar == "esquerda" else config.TECLA_DIREITA,
                                  config.PASSO_ANDAR)
                time.sleep(0.3)
                self.driver.tecla(tecla_lado, 0.05)
                time.sleep(0.3)

        for tecla in estrategia["skills"]:
            self.log(f"usando item {tecla}")
            self.driver.tecla(tecla)
            time.sleep(0.3)

        self.atirar(leitura.ancora, forca)
        self.esperar_fim_da_vez()

    def ajustar_angulo(self, ancora, alvo):
        """Aperta ↑/↓ (um grau por toque) até o número do relógio mostrar o ângulo pedido.

        Só aceita quando lê o número exato. Com um dígito desconhecido, a dezena diz para que
        lado ir; sem número legível, a agulha (aproximada). Retorna False se o número parar
        de mudar antes de chegar (o chão aqui não deixa esse ângulo).
        """
        anterior, parado, atual = None, 0, None
        for _ in range(60):
            frame = self.driver.capturar()
            atual, exato = ler_numero(frame, ancora)
            if atual is None:
                atual = ler_angulo(frame, ancora)
                if atual is None:
                    self.log("não consegui ler o ângulo; atirando com o que estiver")
                    return True
                if abs(alvo - atual) <= config.ANGULO_TOLERANCIA:
                    self.log(f"ângulo pela agulha ~{atual:.0f} (número ilegível)")
                    return True
            elif exato and atual == alvo:
                self.log(f"ângulo {atual}")
                return True
            # Travou = o número não muda em 4 toques seguidos.
            mascara = mascara_numero(frame, ancora)
            if anterior is not None and mascara.shape == anterior.shape \
                    and np.count_nonzero(mascara != anterior) <= 3:
                parado += 1
                if parado >= 4:
                    self.log(f"ângulo travou em {atual:.0f}")
                    return False
            else:
                parado = 0
            anterior = mascara
            self.driver.tecla(config.TECLA_ANGULO_SOBE if alvo > atual else config.TECLA_ANGULO_DESCE, 0.06)
            # Dá tempo do jogo redesenhar o relógio antes de ler de novo.
            time.sleep(0.12)
        self.log(f"ângulo não chegou em {alvo} (último lido {atual:.0f})")
        return False

    def atirar(self, ancora, forca):
        """Segura o Espaço e solta quando a barra chega na força pedida."""
        ax, ay = ancora
        regiao = (ax + BARRA_X0, ay + BARRA_Y - 2, BARRA_X100 - BARRA_X0 + 1, 5)
        referencia = self.driver.capturar(regiao)
        limite = time.time() + config.TEMPO_MAX_FORCA
        atual = 0.0
        self.driver.segurar(config.TECLA_ATIRAR)
        try:
            while time.time() < limite:
                atual = forca_na_barra(referencia, self.driver.capturar(regiao))
                if atual >= forca - config.FORCA_ANTECIPACAO:
                    break
        finally:
            self.driver.soltar(config.TECLA_ATIRAR)
        self.log(f"tiro: pedida {forca:.0f}, lida ao soltar {atual:.0f}")

    def esperar_fim_da_vez(self):
        """Espera o PASS sumir (a vez passou para os inimigos)."""
        limite = time.time() + 10
        while time.time() < limite:
            if vision.achar(PASS, self.driver.capturar(), LIMIAR_PASS) is None:
                return
            time.sleep(0.3)
