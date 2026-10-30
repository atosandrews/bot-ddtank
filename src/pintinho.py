"""Pintinho Corre Rápido (Difícil): uma conta faz a instância inteira, em ciclo sem parar.

Uso: python -m src.pintinho          (ou rodar_pintinho.bat; clique na janela da conta ao iniciar)
     python -m src.pintinho HWND     (número da janela; ver tools/listar_janelas.py)

Separado do Formigueiro (src/grupo.py): só reaproveita a leitura da batalha, o ajuste de
ângulo, o tiro e o captcha. Regras do usuário e medidas da gravação de 30/10 em
docs/PINTINHO.md. O estágio vem do quadro de missão, não da ordem das lutas: o bot pode ser
ligado no meio da instância.

- Sala: escolhe Pintinho Corre Rápido / Difícil e inicia (confirma o "lutar sozinho").
- Começo da luta: fecha o popup "Requisitos/Vitória".
- Todo tiro: itens 3 (+2) e 9 (dano máximo). No 1º turno dos estágios 1 e 3 ele voa (F).
- Estágios 2 e 4 (gaiola): força pela tabela do jogo, com a distância do minimapa e o vento.
- Estágio 3: depois do voo recua um pouco e atira 30/20 para o lado da casa. Estágio 4: 20/75.
- Fim de cada estágio: vira 1 carta; no fim da instância, 2 (as grátis). A sala volta sozinha.
"""
import random
import sys
import time
from contextlib import contextmanager
from dataclasses import dataclass

import cv2
import numpy as np

from . import batalha, config, telas, verificacao, vision
from . import pintinho_visao as pv
from .batalha import LIMIAR_PASS, PASS, TABELAS, Batalha, forca_para, lado_virado, ler, ler_numero
from .bot import Bot
from .drivers import DriverTela
from .telas import IMG, Tela

SKILLS = ["3", "9"]  # +2 e dano máximo em todo tiro (usuário, 30/10)
TECLA_VOAR = "f"     # Voar: 1º turno dos estágios 1 e 3, no lugar dos itens
# Força: o jogo ainda enche a barra enquanto a tecla sobe. O bot mede a velocidade da barra e
# solta quando ela vai chegar na força pedida dali a ATRASO s; o atraso se corrige sozinho pela
# força que ficou desenhada depois de cada tiro. No 8º teste (30/10) a folga fixa em força
# oscilava ±2 (pediu 65, saiu 63,4; pediu 75, saiu 76,9).
ATRASO_INICIAL = 0.03
ATRASO_MIN, ATRASO_MAX = 0.0, 0.12
# Fora da vez: confere o ângulo do próximo tiro e corrige de novo a cada isso (s).
PREPARO_INTERVALO = 1.0
# Dígitos do relógio de ângulo que img/digitos/ não tem (lá só 0 a 5): 6 e 9 recortados da
# gravação de 30/10 (relógio em 36 e 39). Um ângulo com dígito ainda desconhecido (ex.: 28) é
# alcançado contando toques a partir do vizinho legível (29) e o dígito novo é salvo aqui.
PASTA_DIGITOS = pv.PASTA / "digitos"

# Estágio 1 (pintinhos): no começo eles estão longe; depois vêm até o personagem.
PERTO_PX = 250            # inimigo na tela a até isso: tiro curto
TIRO_LONGE = (40, 45)     # voando (F) para perto deles (usuário, 30/10: sempre 40/45)
TIRO_PERTO = (40, 10)
CENTRO_DA_TELA = 349      # x do meio da área da luta em relação ao rótulo "Força"
# Pintinho quase colado (ou a barra dele escondida atrás da minha): força 5, como no
# Formigueiro (usuário, 30/10).
TIRO_COLADO = (40, 5)
COLADO_PX = 30
# Estágio 2 (3 galinhas à direita): o contador "n/3" diz qual é a próxima. Pedido do usuário
# (30/10, depois do grupo): nasça onde nascer, o personagem vai primeiro ao pixel de trás (o da
# gravação e do print dele: a galinha a essa distância no minimapa) e atira de lá.
# (ângulo, força, distância no minimapa da galinha no pixel de trás)
GALINHAS = [(25, 65, 114), (25, 100, 116), (45, 75, 121)]  # de baixo, do meio, de cima
FORCA_MAXIMA = 100
# Ida ao pixel de trás: passinhos para trás (esquerda) até a galinha ficar à distância gravada.
PIXEL_TOLERANCIA = 1      # px do minimapa (~12 px da tela)
PIXEL_TOQUE = 0.15        # segundos de seta em cada passinho
PIXEL_TEMPO_MAX = 6.0     # s andando, no máximo
# Vento a favor tira 0,17 por unidade (ajuste das 3 voltas da gravação). Vento contra soma
# 0,4: no 3º teste no jogo (30/10) o 1º tiro na do meio (62,4, vento −2,5) e na de cima
# (70,0, vento −3,0) caíram curtos; na gravação o usuário matou com 63 e 71 (−2,5 e −2,4).
FORCA_POR_VENTO = 0.17    # a favor
FORCA_POR_VENTO_CONTRA = 0.4
# Regra do usuário (30/10), "apenas nessa ocasião": nascendo no pixel de trás (o da gravação: a
# galinha a pelo menos a distância gravada no minimapa, 2 px de folga) e vento de 3 ou mais
# para a esquerda, força + 5 (no lugar da correção normal do vento).
VENTO_FORTE_CONTRA = 3.0
FORCA_A_MAIS_VENTO_FORTE = 5
FOLGA_PIXEL_DE_TRAS = 2
FORCA_VENTO_MAX = 3       # nunca corrige mais do que isso (leitura errada do vento)
# Errar uma galinha faz elas porem escudo por um tempo (usuário, 30/10): o contador e o vento
# são lidos em vários prints e o ângulo tem que estar exato no relógio.
LEITURAS_ESTAGIO_2 = 8
# Estágio 3 (Grange, à esquerda): 1º turno voa (F) com 40/60 — o F primeiro, porque voando o
# ângulo sobe mais. No turno seguinte anda um pouco para trás (direita) para pegar a galinha e
# a casa no mesmo tiro, e daí em diante atira sempre para o lado da casa (esquerda) com 30/20
# (usuário, 30/10). Na gravação o recuo foi de 0,42–0,55 s nas 3 voltas.
GRANGE_PRIMEIRO = (40, 60)
GRANGE_DEPOIS = (30, 20)
# Recuo medido no minimapa (px), só no 1º turno depois do voo: na gravação o usuário recuou
# 9, 7 e 5 px. No 4º teste (30/10) um aperto às cegas de 0,45 s quase não andou, o
# personagem ficou colado na casa, se acertou junto e morreu.
GRANGE_RECUO = 7
RECUO_TOQUE = 0.15        # segundos de seta em cada passinho do recuo
RECUO_TEMPO_MAX = 4.0
# Estágio 4: o Burrinho (à esquerda) protege a gaiola com um escudo. Sempre 20/75 nele até
# morrer; sem o ângulo 20 ali, anda só um pouco até ter (usuário, 30/10).
BURRINHO = (20, 75)
# Escudo caiu ("Excelente! O escudo de proteção sumiu"): a gaiola, à direita (ponto verde no
# minimapa). Tabela de 30° pela distância + 5,8: as 3 voltas deram +5,3, +6,3 e +5,9.
# (No Burrinho e no Grange a tabela não bate com os tiros que acertaram: alvos grandes.)
GAIOLA_ANGULO = 30
GAIOLA_A_MAIS = 5.8
GAIOLA_SEM_MAPA = 35      # sem o minimapa: a força média da gravação
GAIOLA_TENTATIVAS = [0, 3, -3, 6]  # errou: a próxima muda um pouco a força
# 7º e 8º testes (30/10): nascendo colado no Burrinho o ângulo não desce de 26; passinhos curtos
# não saíam do lugar e a busca lenta (3 s por passo) fazia a vez acabar sem tiro. O 20 apareceu
# depois de ~1,2 s andando para trás (direita). Agora passos de 0,4 s com teste rápido.
ANDAR_TOQUE = 0.4         # segundos de seta para trás em cada passo
ANDAR_TESTE_TOQUES = 8    # toques rápidos de ↓ para ver se o ângulo já desce
ANDAR_ATE = 7.0           # s desde o começo da vez: passou disso, atira com o ângulo que tiver

CARTAS_POR_TELA = {"estagio": 1, "final": 2}

# As telas fora da batalha são as do bot normal; só a escolha da instância muda.
SELECAO = [pv.PASTA / "selecao_pintinho.png", pv.PASTA / "selecao_dificil.png",
           IMG / "botao_ok_instancia.png"]
TELAS = [t for t in telas.TELAS if t.nome != "SELECAO_INSTANCIA"]
TELAS.insert(next(i for i, t in enumerate(TELAS) if t.nome == "SALA_SEM_INSTANCIA"),
             Tela("SELECAO_INSTANCIA", IMG / "config_sala_titulo.png"))


def identificar(frame):
    for tela in TELAS:
        if vision.achar(tela.reconhecer, frame, config.LIMIAR):
            return tela
    return None


_LER = object()


def com_vento(forca, frame, ancora, lado, vento=_LER):
    """Vento a favor (soprando para o lado do tiro) pede menos força; contra, mais."""
    if vento is _LER:
        vento = pv.ler_vento(frame, ancora)
    if vento is None:
        return forca, None
    a_favor = vento * lado  # positivo: soprando para o lado do tiro
    por_unidade = FORCA_POR_VENTO if a_favor > 0 else FORCA_POR_VENTO_CONTRA
    ajuste = max(-FORCA_VENTO_MAX, min(FORCA_VENTO_MAX, -por_unidade * a_favor))
    return forca + ajuste, vento


def forca_tabela(angulo, distancia):
    """Força da tabela da moldura; entre dois ângulos da tabela, interpola."""
    if angulo in TABELAS:
        return forca_para(angulo, distancia)
    menor = max(a for a in TABELAS if a < angulo)
    maior = min(a for a in TABELAS if a > angulo)
    t = (angulo - menor) / (maior - menor)
    return forca_para(menor, distancia) * (1 - t) + forca_para(maior, distancia) * t


def distancia_minimapa(eu, alvo):
    """Distância horizontal (unidades das tabelas) entre dois x do minimapa."""
    return abs(alvo - eu) * pv.DISTANCIA_POR_PX_MINIMAPA


def galinha_no_minimapa(pontos, mortas):
    """x da galinha da vez: a mais baixa das que sobraram (morrem de baixo para cima)."""
    if pontos is None or pontos[0] is None:
        return None
    vivas = sorted(pontos[1], key=lambda p: -p[1])
    if len(vivas) != 3 - mortas:
        return None  # sobrou outra coisa vermelha ou faltou uma: não confio na posição
    return vivas[0][0]


@dataclass
class Tiro:
    lado: int                 # 1 = direita, -1 = esquerda
    angulo: int
    forca: float
    motivo: str
    andar: bool = False       # sem o ângulo aqui, anda até ter (estágio 4)
    voar: bool = False        # Voar (F): o personagem vai junto com o tiro, sem itens
    galinha: int = None       # estágio 2: quantas já estavam mortas quando mirou
    gaiola: bool = False
    recuar: int = 0           # px do minimapa para andar para trás antes de mirar


def plano(estagio, frame, leitura, tiros_no_estagio, tiros_na_gaiola=0, mortas=None, vento=_LER):
    """O próximo tiro. `mortas` e `vento`: já lidos no estágio 2 (senão lê aqui)."""
    ancora = leitura.ancora
    if estagio == 2:
        if mortas is None:
            mortas = pv.galinhas_mortas(frame, ancora)
        if mortas is None:
            mortas = min(tiros_no_estagio, 2)  # contador ilegível: segue a ordem dos tiros
        angulo, forca, dx_de_tras = GALINHAS[mortas]
        pontos = pv.pontos_minimapa(frame, ancora)
        x = galinha_no_minimapa(pontos, mortas)
        if x is not None:
            de_tras = abs(x - pontos[0]) >= dx_de_tras - FOLGA_PIXEL_DE_TRAS
            onde = f"distância {distancia_minimapa(pontos[0], x):.1f}, " + (
                "pixel de trás" if de_tras else "mais à frente")
        else:
            de_tras, onde = False, "pixel ? (minimapa ilegível)"
        if vento is _LER:
            vento = pv.ler_vento(frame, ancora)
        nome = ("de baixo", "do meio", "de cima")[mortas]
        if de_tras and vento is not None and vento <= -VENTO_FORTE_CONTRA:
            return Tiro(1, angulo, min(FORCA_MAXIMA, forca + FORCA_A_MAIS_VENTO_FORTE),
                        f"galinha {nome} ({mortas}/3 mortas), {onde}, vento forte {vento}: "
                        f"+{FORCA_A_MAIS_VENTO_FORTE}", galinha=mortas)
        forca, vento = com_vento(forca, frame, ancora, 1, vento)
        return Tiro(1, angulo, min(FORCA_MAXIMA, forca),
                    f"galinha {nome} ({mortas}/3 mortas), {onde}, vento {vento}", galinha=mortas)
    if estagio == 3:
        if tiros_no_estagio == 0:
            lado, como = lado_do_centro(frame, leitura, padrao=-1)
            return Tiro(lado, *GRANGE_PRIMEIRO, f"Grange: voando para o centro do mapa ({como})", voar=True)
        if tiros_no_estagio == 1:
            return Tiro(-1, *GRANGE_DEPOIS, "Grange: recuando para pegar galinha e casa",
                        recuar=GRANGE_RECUO)
        return Tiro(-1, *GRANGE_DEPOIS, "Grange e casa")
    if estagio == 4:
        if tiros_no_estagio > 0 and not pv.chefe_vivo(frame, ancora):
            pontos = pv.pontos_minimapa(frame, ancora)
            if pontos and pontos[0] is not None and pontos[2]:
                distancia = distancia_minimapa(pontos[0], pontos[2][0][0])
                forca = forca_tabela(GAIOLA_ANGULO, distancia) + GAIOLA_A_MAIS
                onde = f"distância {distancia:.1f}"
            else:
                forca, onde = GAIOLA_SEM_MAPA, "sem minimapa"
            forca += GAIOLA_TENTATIVAS[tiros_na_gaiola % len(GAIOLA_TENTATIVAS)]
            forca, vento = com_vento(forca, frame, ancora, 1)
            return Tiro(1, GAIOLA_ANGULO, forca, f"escudo caiu: gaiola (tentativa {tiros_na_gaiola + 1}), "
                                                 f"{onde}, vento {vento}", gaiola=True)
        return Tiro(-1, *BURRINHO, "Burrinho", andar=True)
    # Estágio 1 (ou desconhecido). 1º turno: os pintinhos estão longe; o personagem voa (F)
    # para perto deles, para o lado deles no minimapa. Depois eles vêm até o personagem:
    # sempre o tiro curto no mais perto (a barra deles às vezes some atrás da minha; aí vale
    # o lado do minimapa).
    perto = None
    if leitura.eu and leitura.inimigos:
        ex = leitura.eu[0]
        alvo = min(leitura.inimigos, key=lambda p: abs(p[0] - ex))
        perto = (1 if alvo[0] > ex else -1, abs(alvo[0] - ex))
    if perto and perto[1] <= COLADO_PX:
        return Tiro(perto[0], *TIRO_COLADO, f"pintinho quase colado ({perto[1]} px)")
    if perto and (perto[1] <= PERTO_PX or tiros_no_estagio > 0):
        return Tiro(perto[0], *TIRO_PERTO, f"pintinho a {perto[1]} px")
    if tiros_no_estagio > 0:
        # Nenhuma barra à vista: a dele está escondida atrás da minha (colado em mim).
        lado = pv.lado_dos_inimigos(frame, ancora) or (perto[0] if perto else None)
        return Tiro(lado or 1, *TIRO_COLADO, "pintinho colado (sem barra à vista)")
    lado, como = lado_do_centro(frame, leitura)
    return Tiro(lado, *TIRO_LONGE, f"voando para o centro do mapa ({como})", voar=True)


def lado_do_centro(frame, leitura, padrao=1):
    """Lado do centro do mapa, para onde o voo vai sempre (usuário, 30/10: no 5º teste nasceu na
    direita e o voo pelo lado dos pintinhos no minimapa foi errado). Pelo minimapa; sem ele,
    pela posição na tela."""
    lado = pv.lado_do_mapa(frame, leitura.ancora)
    if lado is not None:
        return lado, "pelo minimapa"
    if leitura.eu:
        return (-1 if leitura.eu[0] - leitura.ancora[0] > CENTRO_DA_TELA else 1), "pela tela"
    return padrao, "sem saber: padrão"


def carregar_digitos():
    """Os dígitos do relógio do bot (src/batalha.py) mais os do Pintinho (só neste processo)."""
    digitos = batalha._carregar_digitos()
    for arq in sorted(PASTA_DIGITOS.glob("*.png")):
        digitos[f"{arq.stem}_pintinho"] = cv2.imread(str(arq), cv2.IMREAD_GRAYSCALE)
    return digitos


LIMIAR_RELOGIO = 0.7  # o "0" verde do 40 deu 0,73 (o bot pede 0,75)
SOBRA_RELOGIO = 6     # px de dígito que nenhum modelo explicou e ainda conta como exato


def ler_relogio(frame, ancora):
    """Número do relógio de ângulo: (valor, exato) como batalha.ler_numero, mas só na faixa
    dos dígitos (sujeira acima/abaixo não conta) e com os dígitos do Pintinho.

    O leitor do bot devolvia nada para 35 e 36 (um risco claro na região) e 45 para 40.
    """
    mascara = batalha.mascara_numero(frame, ancora)
    if not mascara.any():
        return None, False
    achados = []
    for nome, tpl in carregar_digitos().items():
        if tpl.shape[0] > mascara.shape[0] or tpl.shape[1] > mascara.shape[1]:
            continue
        res = cv2.matchTemplate(mascara, tpl, cv2.TM_CCOEFF_NORMED)
        for y, x in zip(*np.where(res >= LIMIAR_RELOGIO)):
            achados.append((float(res[y, x]), int(x), int(y), nome[0], tpl.shape[1], tpl.shape[0]))
    escolhidos = []
    for achado in sorted(achados, reverse=True):
        _, x, _, _, w, _ = achado
        if all(min(x + w, x2 + w2) - max(x, x2) <= 2 for _, x2, _, _, w2, _ in escolhidos):
            escolhidos.append(achado)
    if not escolhidos:
        return None, False
    escolhidos.sort(key=lambda e: e[1])
    y0, altura = escolhidos[0][2], escolhidos[0][5]
    faixa = mascara[y0:y0 + altura] > 0
    explicado = np.zeros(faixa.shape[1], bool)
    for _, x, _, _, w, _ in escolhidos:
        # 1 px de folga: em algumas cores o contorno sai 1 px mais largo que o modelo
        # (28 laranja no 7º teste, 30/10: ilegível e o bot passou a vez na galinha).
        explicado[max(x - 1, 0):x + w + 1] = True
    sobra = int(faixa[:, ~explicado].sum())
    texto = "".join(e[3] for e in escolhidos)
    if sobra <= SOBRA_RELOGIO and len(texto) <= 2:
        return int(texto), True
    valor, exato = ler_numero(frame, ancora)  # o leitor do bot (o original, não o trocado)
    if exato:
        return valor, True  # na gravação: 2 prints (20 e 26) que só ele leu
    cols = np.where(faixa.any(axis=0))[0]
    if len(texto) == 1 and cols.max() - cols.min() > 12 and escolhidos[0][1] <= cols.min() + 2:
        return int(texto) * 10 + 5, False  # dezena conhecida, unidade desconhecida (direção)
    return None, False


def angulo_legivel(angulo):
    """O relógio mostra esse ângulo com dígitos que o bot conhece?"""
    conhecidos = {nome[0] for nome in carregar_digitos()}
    return all(d in conhecidos for d in str(angulo))


def unidade_do_numero(mascara, dezena):
    """Recorte (13 px de altura) do dígito das unidades, logo à direita da dezena conhecida."""
    tpl = carregar_digitos().get(dezena)
    if tpl is None or tpl.shape[0] > mascara.shape[0] or tpl.shape[1] > mascara.shape[1]:
        return None
    res = cv2.matchTemplate(mascara, tpl, cv2.TM_CCOEFF_NORMED)
    _, nota, _, (x, y) = cv2.minMaxLoc(res)
    if nota < batalha.LIMIAR_DIGITO:
        return None
    resto = mascara[y:y + tpl.shape[0], x + tpl.shape[1]:]
    cols = np.where(resto.any(axis=0))[0]
    if len(cols) == 0:
        return None
    glifo = resto[:, cols.min():]
    vazias = np.where(~glifo.any(axis=0))[0]
    return glifo[:, :vazias[0]] if len(vazias) else glifo


def soltar_teclas():
    """Solta as teclas do tiro, do andar e do ângulo (passo interrompido no meio)."""
    import pyautogui
    falha = pyautogui.FAILSAFE
    pyautogui.FAILSAFE = False
    try:
        for tecla in (config.TECLA_ATIRAR, config.TECLA_ESQUERDA, config.TECLA_DIREITA,
                      config.TECLA_ANGULO_SOBE, config.TECLA_ANGULO_DESCE):
            pyautogui.keyUp(tecla)
    finally:
        pyautogui.FAILSAFE = falha


@contextmanager
def tolerar_print_invalido(log):
    """Print vazio ou cortado da janela (minimizada ou sumindo por um instante): o grupo caiu
    com isso no 1º teste (30/10, cv2.error no ajuste do ângulo). Avisa, solta as teclas,
    espera e deixa o próximo passo tentar de novo."""
    try:
        yield
    except cv2.error as exc:
        log(f"print inválido da janela (minimizada?): {str(exc).strip().splitlines()[-1][:90]}; "
            f"tentando de novo")
        soltar_teclas()
        time.sleep(1.0)


class BatalhaPintinho(Batalha):
    def __init__(self, driver, log, salvar_debug):
        super().__init__(driver, log, salvar_debug, estrategias={})
        carregar_digitos()
        self.estagio = None
        self.erros_galinha = 0  # só para o log: quantas vezes errou uma galinha
        self.preparado_em = 0.0
        self.inicio_vez = None  # quando a vez atual começou (o andar não pode passar do tempo)
        self.popup_fechado = False
        self.atraso = ATRASO_INICIAL  # s entre ler a barra e o jogo parar de encher
        self._zerar_estagio()

    def _zerar_estagio(self):
        self.tiros_no_estagio = 0
        self.tiros_na_gaiola = 0
        self.galinha_mirada = None  # estágio 2: contador quando atirou (para ver se acertou)

    def nova_luta(self):
        super().nova_luta()
        self._zerar_estagio()
        self.popup_fechado = False

    def ler_estagio(self, frame, leitura):
        estagio = pv.ler_estagio(frame, leitura.ancora)
        if estagio is not None and estagio != self.estagio:
            self.estagio = estagio
            self._zerar_estagio()
            self.virado = None
            self.log(f"estágio {estagio}")
        return self.estagio

    def preparar_angulo(self, frame, leitura=None):
        """Fora da vez: deixa o ângulo do próximo tiro pronto (usuário, 30/10: não perder tempo
        subindo quando a vez chega). Confere de novo a cada PREPARO_INTERVALO até o relógio
        mostrar o ângulo exato: antes da vez as setas às vezes não pegam (animação do inimigo)."""
        leitura = leitura or ler(frame)
        if (leitura is None or leitura.minha_vez or not config.AJUSTAR_ANGULO
                or self.estagio is None or time.time() - self.preparado_em < PREPARO_INTERVALO):
            return False
        tiro = plano(self.estagio, frame, leitura, self.tiros_no_estagio, self.tiros_na_gaiola)
        valor, exato = ler_relogio(frame, leitura.ancora)
        if exato and valor == tiro.angulo:
            return False
        self.preparado_em = time.time()
        if self.angulo_preparado != tiro.angulo:
            self.angulo_preparado = tiro.angulo
            self.log(f"ajustando o ângulo para {tiro.angulo} antes da vez")
        self.ajustar_angulo(leitura.ancora, tiro.angulo)
        return True

    def ler_com_calma(self, leitor, ancora, tentativas=LEITURAS_ESTAGIO_2):
        """Lê de novo em prints seguidos até sair um valor (animação ou balão por cima)."""
        for _ in range(tentativas):
            valor = leitor(self.driver.capturar(), ancora)
            if valor is not None:
                return valor
            time.sleep(0.25)
        return None

    def conferir_galinha(self, ancora):
        """Estágio 2, começo da vez: o contador. Se ele não subiu desde o último tiro, errei:
        a galinha põe escudo e depois tira (usuário, 30/10); o bot mira nela de novo."""
        mortas = self.ler_com_calma(pv.galinhas_mortas, ancora)
        if self.galinha_mirada is not None and mortas == self.galinha_mirada:
            self.erros_galinha += 1
            self.log(f"a galinha não morreu ({mortas}/3 continua, {self.erros_galinha}ª vez): mirando nela "
                     f"de novo (se pôs escudo, ele some sozinho)")
            self.salvar_debug(self.driver.capturar(), "errou_galinha")
        elif mortas is None:
            self.log("não li o contador de galinhas; seguindo a ordem dos tiros")
            self.salvar_debug(self.driver.capturar(), "sem_contador")
        return mortas

    def fechar_popup_missao(self, frame):
        """Começo da luta: fecha o quadro "Requisitos/Vitória" (cobre o meio do campo)."""
        x = pv.x_do_popup_missao(frame)
        if x:
            self.log("fechando o quadro Requisitos/Vitória")
            self.driver.clicar(*x)
            time.sleep(0.4)
            frame = self.driver.capturar()
        self.popup_fechado = x is not None or self.turno > 0
        return frame

    def passo(self, frame):
        leitura = ler(frame)
        if leitura is None:
            time.sleep(0.25)
            return
        if not self.popup_fechado:
            frame = self.fechar_popup_missao(frame)
            leitura = ler(frame) or leitura
        estagio = self.ler_estagio(frame, leitura)
        if not leitura.minha_vez:
            if not self.preparar_angulo(frame, leitura):
                time.sleep(0.25)
            return
        self.turno += 1
        self.inicio_vez = time.time()
        if estagio is None:
            self.log("não li o estágio no quadro de missão; jogando como estágio 1")
            self.salvar_debug(frame, "sem_estagio")
        mortas, vento = None, _LER
        if estagio == 2:
            mortas = self.conferir_galinha(leitura.ancora)
            self.ir_ao_pixel_de_tras(leitura.ancora, mortas)
            vento = self.ler_com_calma(pv.ler_vento, leitura.ancora)
            if vento is None:
                self.log("não li o vento; atirando sem corrigir")
                self.salvar_debug(self.driver.capturar(), "sem_vento")
            frame = self.driver.capturar()
        tiro = plano(estagio, frame, leitura, self.tiros_no_estagio, self.tiros_na_gaiola, mortas, vento)
        self.log(f"turno {self.turno}: {tiro.motivo}")
        if not self.disparar(leitura, tiro):
            return
        self.tiros_no_estagio += 1
        self.tiros_na_gaiola += tiro.gaiola
        if tiro.galinha is not None:
            self.galinha_mirada = tiro.galinha

    def ir_ao_pixel_de_tras(self, ancora, mortas):
        """Estágio 2: anda para trás (esquerda) até a galinha da vez ficar, no minimapa, à
        distância do pixel de trás; passou do ponto, volta. Depois fica virado para trás (o
        disparo vira para as galinhas)."""
        indice = min(mortas if mortas is not None else self.tiros_no_estagio, 2)
        dx_alvo = GALINHAS[indice][2]

        def falta():
            pontos = pv.pontos_minimapa(self.driver.capturar(), ancora)
            x = galinha_no_minimapa(pontos, indice)
            return None if x is None else dx_alvo - (x - pontos[0])  # > 0: estou perto demais

        agora = falta()
        if agora is None:
            self.log("não achei a galinha no minimapa; atirando de onde estou")
            return
        if abs(agora) <= PIXEL_TOLERANCIA:
            return
        self.log(f"indo ao pixel de trás: {agora:+d} px no minimapa")
        limite = time.time() + PIXEL_TEMPO_MAX
        while time.time() < limite and agora is not None and abs(agora) > PIXEL_TOLERANCIA:
            tecla = config.TECLA_ESQUERDA if agora > 0 else config.TECLA_DIREITA
            self.driver.tecla(tecla, PIXEL_TOQUE)
            self.virado = -1 if agora > 0 else 1
            time.sleep(0.12)
            agora = falta()
        self.log(f"no pixel de trás (falta {agora} px)" if agora is not None and abs(agora) <= PIXEL_TOLERANCIA
                 else f"não cheguei ao pixel de trás (falta {agora} px)")

    def angulo_exato(self, ancora, angulo):
        """O número do relógio mostra exatamente o ângulo pedido (sem aproximação)?"""
        valor, exato = ler_relogio(self.driver.capturar(), ancora)
        return exato and valor == angulo

    def passar_a_vez(self, motivo):
        self.log(f"{motivo}: passando a vez em vez de arriscar o tiro")
        self.salvar_debug(self.driver.capturar(), "passou")
        pos = vision.achar(PASS, self.driver.capturar(), LIMIAR_PASS)
        if pos:
            self.driver.clicar(*pos)
        self.esperar_fim_da_vez()

    def disparar(self, leitura, tiro):
        lado = tiro.lado
        self.log(f"{'voar' if tiro.voar else 'tiro'} para a {'direita' if lado > 0 else 'esquerda'}: "
                 f"ângulo {tiro.angulo}, força {tiro.forca:.1f}")
        tecla_lado = config.TECLA_DIREITA if lado > 0 else config.TECLA_ESQUERDA
        frame = self.driver.capturar()
        virado = lado_virado(frame, leitura.ancora)
        self.registrar_tiro(frame, leitura, lado, virado, virar=(virado or self.virado) != lado)
        if virado is not None:
            self.virado = virado
        if tiro.voar:
            # Primeiro o F: voando, o ângulo sobe mais do que no chão (usuário, 30/10).
            self.driver.tecla(TECLA_VOAR)
            time.sleep(random.uniform(0.15, 0.25))
        if tiro.recuar:
            self.recuar(leitura.ancora, lado, tiro.recuar)
        if self.virado != lado:
            self.driver.tecla(tecla_lado, 0.05)
            self.virado = lado
            time.sleep(random.uniform(0.08, 0.14))
        if config.AJUSTAR_ANGULO and not self.ajustar_angulo(leitura.ancora, tiro.angulo) and tiro.andar:
            self.andar_ate_angulo(leitura, lado, tiro.angulo)
        if tiro.galinha is not None:
            # Errar uma galinha trava o estágio: só atira com o ângulo exato no relógio.
            for _ in range(2):
                if self.angulo_exato(leitura.ancora, tiro.angulo):
                    break
                self.log(f"o relógio não mostra {tiro.angulo} exato; ajustando de novo")
                self.ajustar_angulo(leitura.ancora, tiro.angulo)
            else:
                if not self.angulo_exato(leitura.ancora, tiro.angulo):
                    self.passar_a_vez(f"ângulo {tiro.angulo} não confirmado")
                    return False
        if not tiro.voar:
            for tecla in SKILLS:
                self.driver.tecla(tecla)
                time.sleep(random.uniform(0.12, 0.22))
        self.atirar(leitura.ancora, tiro.forca)
        self.esperar_fim_da_vez()
        return True

    def ajustar_angulo(self, ancora, alvo):
        """Como o do bot; se o relógio não sabe mostrar o alvo (dígito desconhecido), vai até o
        vizinho legível mais perto e conta os toques, conferindo que o número mudou a cada um.
        Na 1ª vez que chega assim, aprende o dígito novo."""
        if angulo_legivel(alvo):
            return self._ajustar_com_relogio(ancora, alvo)
        vizinhos = sorted((v for v in range(alvo - 3, alvo + 4) if v != alvo and angulo_legivel(v)),
                          key=lambda v: abs(v - alvo))
        if not vizinhos:
            return self._ajustar_com_relogio(ancora, alvo)
        base = vizinhos[0]
        if not self._ajustar_com_relogio(ancora, base):
            return False
        tecla = config.TECLA_ANGULO_SOBE if alvo > base else config.TECLA_ANGULO_DESCE
        for _ in range(abs(alvo - base)):
            antes = batalha.mascara_numero(self.driver.capturar(), ancora)
            self.driver.tecla(tecla, 0.06)
            limite = time.time() + 0.6
            while time.time() < limite:
                time.sleep(0.05)
                depois = batalha.mascara_numero(self.driver.capturar(), ancora)
                if depois.shape != antes.shape or np.count_nonzero(depois != antes) > 3:
                    break
            else:
                self.log(f"o relógio não mudou indo de {base} para {alvo}")
                return False
        time.sleep(0.05)
        valor, exato = ler_relogio(self.driver.capturar(), ancora)
        if exato and valor != alvo:
            self.log(f"contando toques cheguei em {valor}, não em {alvo}")
            return False
        if not exato:
            self.aprender_digito(ancora, alvo)
        self.log(f"ângulo {alvo} (contando {abs(alvo - base)} toque(s) a partir de {base})")
        return True

    def _ajustar_com_relogio(self, ancora, alvo):
        """O ajuste do bot (src/batalha.py) lendo o relógio com ler_relogio."""
        original = batalha.ler_numero
        batalha.ler_numero = ler_relogio
        try:
            return Batalha.ajustar_angulo(self, ancora, alvo)
        finally:
            batalha.ler_numero = original

    def aprender_digito(self, ancora, alvo):
        """Salva o desenho da unidade de `alvo` (ex.: o 8 de 28) para ler o relógio exato."""
        texto = str(alvo)
        if len(texto) != 2 or angulo_legivel(alvo):
            return
        glifo = unidade_do_numero(batalha.mascara_numero(self.driver.capturar(), ancora), texto[0])
        if glifo is None or glifo.shape[0] < 10 or not 4 <= glifo.shape[1] <= 12:
            return
        PASTA_DIGITOS.mkdir(parents=True, exist_ok=True)
        cv2.imwrite(str(PASTA_DIGITOS / f"{texto[1]}_auto.png"), glifo)
        batalha._carregar_digitos()[f"{texto[1]}_auto_pintinho"] = glifo
        self.log(f"aprendi o dígito {texto[1]} do relógio (em {alvo})")

    def recuar(self, ancora, lado, pixels):
        """Anda para trás (o lado oposto ao tiro) em passinhos até o minimapa mostrar que
        recuou `pixels`. Sem o minimapa, anda o tempo que o usuário andou na gravação."""
        tecla_tras = config.TECLA_ESQUERDA if lado > 0 else config.TECLA_DIREITA
        pontos = pv.pontos_minimapa(self.driver.capturar(), ancora)
        inicio_x = pontos[0] if pontos else None
        if inicio_x is None:
            self.log("recuando sem o minimapa: 0,6 s")
            self.driver.tecla(tecla_tras, 0.6)
        else:
            limite, andou = time.time() + RECUO_TEMPO_MAX, 0
            while time.time() < limite:
                self.driver.tecla(tecla_tras, RECUO_TOQUE)
                time.sleep(0.12)
                pontos = pv.pontos_minimapa(self.driver.capturar(), ancora)
                if pontos and pontos[0] is not None:
                    andou = (pontos[0] - inicio_x) * -lado
                    if andou >= pixels:
                        break
            self.log(f"recuei {andou} px no minimapa (pedido {pixels})")
        time.sleep(random.uniform(0.15, 0.25))
        self.virado = -lado

    def atirar(self, ancora, forca):
        """Segura o Espaço e solta quando a barra (lida ao vivo) chega na força pedida.

        Ao apertar, a barra zera e enche de novo: só aceita a leitura depois de vê-la
        recomeçar (antes disso pode ser o desenho do tiro anterior). Depois do tiro lê a força
        que ficou e corrige a antecipação dos próximos (o jogo ainda enche um pouco enquanto a
        tecla sobe).
        """
        regiao = pv.regiao_forca(ancora)
        anterior = pv.forca_na_faixa(self.driver.capturar(regiao))
        limite = time.time() + config.TEMPO_MAX_FORCA
        inicio, atual, recomecou = time.time(), 0.0, anterior <= 3
        amostras = []  # (tempo, força) depois de a barra recomeçar
        velocidade = 0.0
        self.driver.segurar(config.TECLA_ATIRAR)
        try:
            while time.time() < limite:
                atual = pv.forca_na_faixa(self.driver.capturar(regiao))
                agora = time.time()
                if not recomecou:
                    recomecou = atual < anterior - 3 or atual <= 3
                    continue
                amostras.append((agora, atual))
                velha = next((a for a in reversed(amostras) if agora - a[0] >= 0.08), None)
                if velha and atual > velha[1]:
                    velocidade = (atual - velha[1]) / (agora - velha[0])
                if atual + velocidade * self.atraso >= forca:
                    break
                if time.time() - inicio > 1.5 and (atual == 0 or not recomecou):
                    self.log("a barra de força não recomeçou; soltando sem atirar")
                    self.salvar_debug(self.driver.capturar(), "barra_parada")
                    break
        finally:
            self.driver.soltar(config.TECLA_ATIRAR)
        time.sleep(0.12)
        real = pv.forca_na_faixa(self.driver.capturar(regiao))
        if recomecou and real > 3 and velocidade > 0:
            # Saiu a mais: o jogo encheu mais tempo depois da leitura do que o previsto.
            erro_s = (real - forca) / velocidade
            self.atraso = min(ATRASO_MAX, max(ATRASO_MIN, self.atraso + 0.3 * erro_s))
        self.log(f"tiro: pedida {forca:.1f}, saiu {real:.1f} (lida ao soltar {atual:.1f}, "
                 f"barra {velocidade:.0f}/s, atraso agora {self.atraso * 1000:.0f} ms)")
        return real

    def andar_ate_angulo(self, leitura, lado, angulo):
        """O chão aqui não deixa o ângulo (estágio 4: nascendo colado no Burrinho o mínimo é 26).
        Anda para trás (longe do alvo) em passos e, a cada um, testa rápido se o ângulo já
        desce até o pedido; dá, ajusta o exato. Tudo dentro de ANDAR_ATE s da vez.

        No 8º teste (30/10) o 20 só apareceu depois de ~1,2 s andando para a direita.
        """
        self.salvar_debug(self.driver.capturar(), "angulo_travou")
        tecla_tras = config.TECLA_ESQUERDA if lado > 0 else config.TECLA_DIREITA
        tecla_lado = config.TECLA_DIREITA if lado > 0 else config.TECLA_ESQUERDA
        inicio = self.inicio_vez or time.time()
        passo = 0
        while time.time() - inicio < ANDAR_ATE:
            passo += 1
            self.log(f"ângulo {angulo} não dá aqui: andando para trás ({passo})")
            self.driver.tecla(tecla_tras, ANDAR_TOQUE)
            time.sleep(0.15)
            self.driver.tecla(tecla_lado, 0.05)
            self.virado = lado
            time.sleep(0.15)
            if self.angulo_desce_ate(leitura.ancora, angulo):
                return self.ajustar_angulo(leitura.ancora, angulo)
        self.log("o tempo da vez está acabando: atirando com o ângulo que deu")
        return False

    def angulo_desce_ate(self, ancora, angulo):
        """Teste rápido: uma rajada de ↓ e o relógio já chega a (perto de) `angulo`?"""
        for _ in range(ANDAR_TESTE_TOQUES):
            self.driver.tecla(config.TECLA_ANGULO_DESCE, 0.04)
            time.sleep(0.03)
        time.sleep(0.3)
        valor, _ = ler_relogio(self.driver.capturar(), ancora)
        return valor is not None and valor <= angulo + 1


class BotPintinho(Bot):
    def __init__(self, driver, nome="pintinho"):
        super().__init__(driver, nome)
        self.batalha = BatalhaPintinho(driver, self.log, self.salvar_debug)
        self.cartas_viradas = 0

    def passo(self):
        with tolerar_print_invalido(self.log):
            return self._passo()
        return True

    def _passo(self):
        frame = self.driver.capturar()
        if verificacao.tem_codigo(frame):
            verificacao.tratar(self.driver, self.nome)
            return True

        tela = identificar(frame)
        # Fora da batalha: a tela de cartas não é nenhuma das telas conhecidas.
        cartas = None if tela is not None and tela.batalha else pv.tela_de_cartas(frame)
        if cartas:
            self.virar_carta(cartas, frame)
            return True
        self.cartas_viradas = 0

        if tela is None:
            self.sem_tela += 1
            if self.sem_tela == config.SEM_TELA_MAX:
                self.log("nenhuma tela conhecida há um tempo")
                self.salvar_debug(frame, "desconhecida")
            self.driver.afastar_mouse()
            time.sleep(0.5)
            return True
        self.sem_tela = 0

        if tela.nome != self.estado:
            self.estado = tela.nome
            self.repeticoes = 0
            self.log("tela reconhecida")
            if tela.batalha:
                self.batalha.nova_luta()
                time.sleep(config.PAUSA_INICIO_BATALHA)
                return True
        else:
            self.repeticoes += 1
            if self.repeticoes == config.REPETICOES_MAX and not tela.batalha:
                self.log("a tela não mudou depois de vários cliques (clique no lugar errado?)")
                self.salvar_debug(frame, "travado")

        if tela.batalha:
            self.batalha.passo(frame)
            return True

        if tela.nome == "SELECAO_INSTANCIA":
            self.escolher_instancia(frame)
            return True

        for clique in tela.cliques:
            pos, frame = self.esperar_alvo(clique.alvo, frame)
            if pos is None:
                if not clique.opcional:
                    self.log(f"alvo {clique.alvo.name} não encontrado")
                    return True
                continue
            x, y = pos[0] + clique.deslocamento[0], pos[1] + clique.deslocamento[1]
            self.log(f"clicando em {clique.nome or clique.alvo.stem}")
            self.driver.clicar(x, y)
            time.sleep(random.uniform(*config.ESPERA_CLIQUE))
            frame = self.driver.capturar()
        time.sleep(random.uniform(*config.ESPERA_TRANSICAO))
        return True

    def escolher_instancia(self, frame):
        """Pintinho → Difícil → ok. Os botões de dificuldade mudam de lugar depois de escolher
        a instância, então cada clique espera a tela assentar e procura o botão de novo."""
        for alvo in SELECAO:
            pos, frame = self.esperar_alvo(alvo, frame)
            if pos is None:
                self.log(f"{alvo.name} não apareceu na Configuração da sala")
                self.salvar_debug(frame, "selecao")
                return
            self.log(f"clicando em {alvo.stem}")
            self.driver.clicar(*pos)
            time.sleep(random.uniform(0.6, 0.9))
            self.driver.afastar_mouse()  # o brilho do mouse em cima muda o botão seguinte
            frame = self.driver.capturar()
        time.sleep(random.uniform(*config.ESPERA_TRANSICAO))

    def virar_carta(self, tipo, frame):
        """Vira as cartas grátis (1 por estágio, 2 no fim); a sala volta sozinha depois."""
        if self.cartas_viradas >= CARTAS_POR_TELA[tipo]:
            time.sleep(0.5)
            return
        fechadas = pv.cartas_fechadas(frame)
        if not fechadas:
            time.sleep(0.3)
            return
        x, y = random.choice(fechadas)
        self.log(f"virando uma carta ({self.cartas_viradas + 1} de {CARTAS_POR_TELA[tipo]})")
        self.driver.clicar(x, y)
        time.sleep(0.8)
        self.driver.afastar_mouse()
        # Só conta se a carta virou mesmo (o clique pode cair durante a animação de entrada).
        if len(pv.cartas_fechadas(self.driver.capturar())) < len(fechadas):
            self.cartas_viradas += 1


def main():
    from .drivers.conta import contas_abertas, esperar_clique_na_principal
    from .drivers.tela import ativar_dpi
    ativar_dpi()
    hwnd = int(sys.argv[1]) if len(sys.argv) > 1 else esperar_clique_na_principal(contas_abertas())
    if hwnd is None:
        print("Nenhuma janela do jogo escolhida.")
        return 1
    return 0 if BotPintinho(DriverTela(hwnd=hwnd)).rodar() else 1


if __name__ == "__main__":
    sys.exit(main())
