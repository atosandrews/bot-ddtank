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
from dataclasses import dataclass

from . import config, telas, verificacao, vision
from . import pintinho_visao as pv
from .batalha import LIMIAR_PASS, PASS, TABELAS, Batalha, forca_para, lado_virado, ler, ler_numero
from .bot import Bot
from .drivers import DriverTela
from .telas import IMG, Tela

SKILLS = ["3", "9"]  # +2 e dano máximo em todo tiro (usuário, 30/10)
TECLA_VOAR = "f"     # Voar: 1º turno dos estágios 1 e 3, no lugar dos itens
# Força: solta o Espaço um pouco antes (o jogo ainda enche enquanto a tecla sobe). Começa no
# valor do config e se corrige sozinho pela força que ficou desenhada depois de cada tiro.
ANTECIPACAO_MIN, ANTECIPACAO_MAX = 0.0, 4.0
# Fora da vez: confere o ângulo do próximo tiro e corrige de novo a cada isso (s).
PREPARO_INTERVALO = 1.0

# Estágio 1 (pintinhos): no começo eles estão longe; depois vêm até o personagem.
PERTO_PX = 250            # inimigo na tela a até isso: tiro curto
TIRO_LONGE = (40, 45)     # voando (F) para perto deles (usuário, 30/10: sempre 40/45)
TIRO_PERTO = (40, 10)
CENTRO_DA_TELA = 349      # x do meio da área da luta em relação ao rótulo "Força"
# Pintinho quase colado (ou a barra dele escondida atrás da minha): força 5, como no
# Formigueiro (usuário, 30/10).
TIRO_COLADO = (40, 5)
COLADO_PX = 30
# Estágio 2 (3 galinhas à direita): o contador "n/3" diz qual é a próxima. A força segue a
# tabela da moldura do jogo pela distância medida no minimapa (o pixel em que nasceu muda),
# partindo do tiro que acertou na gravação: (ângulo, força sem vento, distância no minimapa).
# Ângulo fora da tabela (45, 55) interpola entre as tabelas vizinhas.
# Baixo: o usuário disse 25/60, mas nas voltas 2 e 3 (o padrão) usou 30/62 e acertou as duas.
# As forças sem vento e o vento vêm do ajuste das 3 voltas: a tabela explicou a diferença da
# volta 2 (nasceu 9 px mais perto no minimapa) com erro de até 0,4 de força.
GALINHAS = [(30, 62.2, 114), (45, 62.5, 116), (55, 70.6, 121)]  # de baixo, do meio, de cima
# Vento a favor tira 0,17 por unidade (ajuste das 3 voltas da gravação). Vento contra soma
# 0,4: no 3º teste no jogo (30/10) o 1º tiro na do meio (62,4, vento −2,5) e na de cima
# (70,0, vento −3,0) caíram curtos; na gravação o usuário matou com 63 e 71 (−2,5 e −2,4).
FORCA_POR_VENTO = 0.17    # a favor
FORCA_POR_VENTO_CONTRA = 0.4
# Regra do usuário (30/10), só neste caso: nascendo no pixel de trás (o da gravação, a
# galinha a pelo menos a distância gravada no minimapa, com 2 px de folga) e vento de 3 ou
# mais para a esquerda, a força é a da tabela + 5 (no lugar da correção normal do vento).
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
ANDAR_TOQUE = 0.15        # segundos de seta em cada passinho
ANDAR_PASSOS = (3, 6)     # passinhos para trás (longe do alvo) e, se não der, para frente
ANDAR_TEMPO_MAX = 12      # a vez acaba: atira com o que tiver

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
        angulo, forca, dx_gravacao = GALINHAS[mortas]
        pontos = pv.pontos_minimapa(frame, ancora)
        x = galinha_no_minimapa(pontos, mortas)
        pixel_de_tras = x is None  # sem minimapa: vale o lugar da gravação (o de trás)
        if x is not None:
            distancia = distancia_minimapa(pontos[0], x)
            gravacao = dx_gravacao * pv.DISTANCIA_POR_PX_MINIMAPA
            forca += forca_tabela(angulo, distancia) - forca_tabela(angulo, gravacao)
            pixel_de_tras = abs(x - pontos[0]) >= dx_gravacao - FOLGA_PIXEL_DE_TRAS
            onde = f"distância {distancia:.1f}"
        else:
            onde = "distância da gravação (minimapa ilegível)"
        if vento is _LER:
            vento = pv.ler_vento(frame, ancora)
        nome = ("de baixo", "do meio", "de cima")[mortas]
        if pixel_de_tras and vento is not None and vento <= -VENTO_FORTE_CONTRA:
            return Tiro(1, angulo, forca + FORCA_A_MAIS_VENTO_FORTE,
                        f"galinha {nome} ({mortas}/3 mortas), {onde}, pixel de trás com vento {vento}: "
                        f"+{FORCA_A_MAIS_VENTO_FORTE}", galinha=mortas)
        forca, vento = com_vento(forca, frame, ancora, 1, vento)
        return Tiro(1, angulo, forca, f"galinha {nome} ({mortas}/3 mortas), {onde}, vento {vento}",
                    galinha=mortas)
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


class BatalhaPintinho(Batalha):
    def __init__(self, driver, log, salvar_debug):
        super().__init__(driver, log, salvar_debug, estrategias={})
        self.estagio = None
        self.erros_galinha = 0  # só para o log: quantas vezes errou uma galinha
        self.preparado_em = 0.0
        self.antecipacao = config.FORCA_ANTECIPACAO
        self._zerar_estagio()

    def _zerar_estagio(self):
        self.tiros_no_estagio = 0
        self.tiros_na_gaiola = 0
        self.galinha_mirada = None  # estágio 2: contador quando atirou (para ver se acertou)

    def nova_luta(self):
        super().nova_luta()
        self._zerar_estagio()

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
        valor, exato = ler_numero(frame, leitura.ancora)
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

    def passo(self, frame):
        leitura = ler(frame)
        if leitura is None:
            time.sleep(0.25)
            return
        estagio = self.ler_estagio(frame, leitura)
        if not leitura.minha_vez:
            if not self.preparar_angulo(frame, leitura):
                time.sleep(0.25)
            return
        self.turno += 1
        if estagio is None:
            self.log("não li o estágio no quadro de missão; jogando como estágio 1")
            self.salvar_debug(frame, "sem_estagio")
        mortas, vento = None, _LER
        if estagio == 2:
            mortas = self.conferir_galinha(leitura.ancora)
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

    def angulo_exato(self, ancora, angulo):
        """O número do relógio mostra exatamente o ângulo pedido (sem aproximação)?"""
        valor, exato = ler_numero(self.driver.capturar(), ancora)
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
        alvo = forca - self.antecipacao
        limite = time.time() + config.TEMPO_MAX_FORCA
        inicio, atual, recomecou = time.time(), 0.0, anterior <= 3
        self.driver.segurar(config.TECLA_ATIRAR)
        try:
            while time.time() < limite:
                atual = pv.forca_na_faixa(self.driver.capturar(regiao))
                if not recomecou:
                    recomecou = atual < anterior - 3 or atual <= 3
                elif atual >= alvo:
                    break
                if time.time() - inicio > 1.5 and (atual == 0 or not recomecou):
                    self.log("a barra de força não recomeçou; soltando sem atirar")
                    self.salvar_debug(self.driver.capturar(), "barra_parada")
                    break
        finally:
            self.driver.soltar(config.TECLA_ATIRAR)
        time.sleep(0.12)
        real = pv.forca_na_faixa(self.driver.capturar(regiao))
        if recomecou and real > 3:
            erro = real - forca
            # Passo pequeno: a leitura varia ~0,5 por print; com 0,5 a antecipação oscilava.
            self.antecipacao = min(ANTECIPACAO_MAX, max(ANTECIPACAO_MIN, self.antecipacao + 0.3 * erro))
        self.log(f"tiro: pedida {forca:.1f}, saiu {real:.1f} (lida ao soltar {atual:.1f}, "
                 f"antecipação agora {self.antecipacao:.1f})")
        return real

    def andar_ate_angulo(self, leitura, lado, angulo):
        """O chão aqui não deixa o ângulo: passinhos para trás e, se não der, para frente.

        Depois de cada passinho vira de volta para o alvo e tenta o ângulo de novo.
        """
        self.salvar_debug(self.driver.capturar(), "angulo_travou")
        tecla_lado = config.TECLA_DIREITA if lado > 0 else config.TECLA_ESQUERDA
        inicio = time.time()
        for direcao, passos in zip((-lado, lado), ANDAR_PASSOS):
            tecla = config.TECLA_DIREITA if direcao > 0 else config.TECLA_ESQUERDA
            for passo in range(passos):
                if time.time() - inicio > ANDAR_TEMPO_MAX:
                    self.log("o tempo da vez está acabando: atirando com o ângulo que deu")
                    return False
                self.log(f"ângulo {angulo} não dá aqui: andando para a "
                         f"{'direita' if direcao > 0 else 'esquerda'} ({passo + 1})")
                self.driver.tecla(tecla, ANDAR_TOQUE)
                time.sleep(0.25)
                self.driver.tecla(tecla_lado, 0.05)
                self.virado = lado
                time.sleep(0.25)
                if self.ajustar_angulo(leitura.ancora, angulo):
                    return True
        self.log("não achei lugar com esse ângulo; atirando com o que deu")
        return False


class BotPintinho(Bot):
    def __init__(self, driver, nome="pintinho"):
        super().__init__(driver, nome)
        self.batalha = BatalhaPintinho(driver, self.log, self.salvar_debug)
        self.cartas_viradas = 0
        self.popup_fechado = False

    def passo(self):
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
                self.popup_fechado = False
                time.sleep(config.PAUSA_INICIO_BATALHA)
                return True
        else:
            self.repeticoes += 1
            if self.repeticoes == config.REPETICOES_MAX and not tela.batalha:
                self.log("a tela não mudou depois de vários cliques (clique no lugar errado?)")
                self.salvar_debug(frame, "travado")

        if tela.batalha:
            if not self.popup_fechado:
                x = pv.x_do_popup_missao(frame)
                if x:
                    self.log("fechando o quadro Requisitos/Vitória")
                    self.driver.clicar(*x)
                    time.sleep(0.4)
                    frame = self.driver.capturar()
                self.popup_fechado = x is not None or self.batalha.turno > 0
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
