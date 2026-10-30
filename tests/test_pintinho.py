"""Pintinho Corre Rápido: visão e decisões com prints da gravação de 30/10 (sem o jogo).

Os prints em tests/imagens/pintinho/ são recortes (x 400–1480, y 200–880) dos quadros de
logs/gravacao/20261030_100744, cliente 1920x1032. O nome traz o que a tela mostra.
"""
from pathlib import Path

import cv2
import numpy as np
import pytest

from src import batalha, pintinho
from src import pintinho_visao as pv
from src.pintinho import BatalhaPintinho, BotPintinho, Tiro, plano

PASTA = Path(__file__).resolve().parent / "imagens" / "pintinho"
ORIGEM = (400, 200)  # canto do recorte no print original


def img(nome):
    frame = cv2.imread(str(PASTA / f"{nome}.png"))
    assert frame is not None, nome
    return frame


class Driver:
    def __init__(self, *frames):
        self.frames = list(frames)
        self.cliques, self.teclas = [], []

    def capturar(self, sub=None):
        frame = self.frames[0] if len(self.frames) == 1 else self.frames.pop(0)
        if sub:
            x, y, w, h = sub
            return frame[y:y + h, x:x + w]
        return frame

    def clicar(self, x, y):
        self.cliques.append((x, y))

    def tecla(self, nome, segundos=0.05):
        self.teclas.append(nome)

    def afastar_mouse(self):
        pass


@pytest.fixture(autouse=True)
def sem_espera(monkeypatch):
    monkeypatch.setattr(pintinho.time, "sleep", lambda s: None)


@pytest.mark.parametrize("nome, estagio", [
    ("e1", 1), ("e2_0de3_vento-1.2", 2), ("e2_vento+2.7", 2), ("e3", 3), ("e4", 4), ("e4_escudo_caiu", 4),
])
def test_estagio_pelo_quadro_de_missao(nome, estagio):
    frame = img(nome)
    assert pv.ler_estagio(frame, batalha.ler(frame).ancora) == estagio


@pytest.mark.parametrize("nome, mortas", [
    ("e2_0de3_vento-1.2", 0), ("e2_1de3_vento-2.5", 1), ("e2_2de3_vento-2.4", 2),
])
def test_contador_de_galinhas(nome, mortas):
    frame = img(nome)
    assert pv.galinhas_mortas(frame, batalha.ler(frame).ancora) == mortas


@pytest.mark.parametrize("nome, vento", [
    ("e2_0de3_vento-1.2", -1.2), ("e2_1de3_vento-2.5", -2.5), ("e2_2de3_vento-2.4", -2.4),
    ("e2_vento+2.7", 2.7),
    ("e2_vento+1.2", 1.2),  # este print não foi usado para recortar os dígitos
])
def test_vento_com_sinal(nome, vento):
    frame = img(nome)
    assert pv.ler_vento(frame, batalha.ler(frame).ancora) == pytest.approx(vento)


def test_minimapa_mostra_os_pintinhos_a_direita_no_comeco():
    frame = img("e1")
    eu, inimigos = pv.ler_minimapa(frame, batalha.ler(frame).ancora)
    assert eu is not None and len(inimigos) >= 2 and all(x > eu for x in inimigos)
    assert pv.lado_dos_inimigos(frame, batalha.ler(frame).ancora) == 1


def test_barra_do_burrinho_diz_se_o_escudo_caiu():
    vivo, morto = img("e4"), img("e4_escudo_caiu")
    assert pv.chefe_vivo(vivo, batalha.ler(vivo).ancora)
    assert not pv.chefe_vivo(morto, batalha.ler(morto).ancora)


def test_x_do_popup_de_missao():
    x, y = pv.x_do_popup_missao(img("e1_popup"))
    # O usuário clicou em (1148, 484) no print original.
    assert abs(x - (1148 - ORIGEM[0])) <= 4 and abs(y - (484 - ORIGEM[1])) <= 4


def test_cartas_entre_estagios_e_no_fim():
    estagio, final = img("cartas_estagio"), img("cartas_final")
    assert pv.tela_de_cartas(estagio) == "estagio"
    assert pv.tela_de_cartas(final) == "final"
    # Uma carta de cada print estava com o mouse em cima (brilho): pode não contar.
    assert 8 <= len(pv.cartas_fechadas(estagio)) <= 9
    assert 20 <= len(pv.cartas_fechadas(final)) <= 21
    assert pv.tela_de_cartas(img("e1")) is None


def test_fora_das_cartas_e_da_batalha_nada_e_lido():
    frame = img("selecao")
    assert batalha.ler(frame) is None and pv.tela_de_cartas(frame) is None


def test_plano_da_galinha_de_baixo_com_vento_contra():
    frame = img("e2_0de3_vento-1.2")
    tiro = plano(2, frame, batalha.ler(frame), 0)
    assert (tiro.lado, tiro.angulo, tiro.andar, tiro.voar, tiro.galinha) == (1, 30, False, False, 0)
    # Volta 1: nasceu no mesmo pixel da referência; vento −1,2 contra soma 0,17 × 1,2.
    assert tiro.forca == pytest.approx(62.2 + 0.17 * 1.2, abs=0.3)
    assert "distância 13.2" in tiro.motivo


def test_plano_da_galinha_de_cima_com_vento_a_favor():
    frame = img("e2_vento+2.7")
    tiro = plano(2, frame, batalha.ler(frame), 2)
    assert (tiro.lado, tiro.angulo, tiro.galinha) == (1, 55, 2) and "de cima" in tiro.motivo
    # Volta 2: nasceu ~1 de distância mais perto; a tabela tira ~2,5 e o vento a favor 0,46.
    # O usuário acertou com 68.
    assert tiro.forca == pytest.approx(67.1, abs=0.5)


def test_plano_do_estagio_4_burrinho_e_depois_a_gaiola():
    vivo, morto = img("e4"), img("e4_escudo_caiu")
    primeiro = plano(4, vivo, batalha.ler(vivo), 0)
    depois = plano(4, vivo, batalha.ler(vivo), 2)
    gaiola = plano(4, morto, batalha.ler(morto), 4)
    assert (primeiro.lado, primeiro.angulo, primeiro.forca, primeiro.andar) == (-1, 20, 75, True)
    assert (depois.lado, depois.angulo, depois.forca, depois.andar) == (-1, 20, 75, True)
    assert (gaiola.lado, gaiola.angulo, gaiola.andar, gaiola.gaiola) == (1, 30, False, True)


def test_estagios_1_e_3_comecam_voando():
    frame = img("e1")
    leitura = batalha.ler(frame)
    leitura.inimigos = []  # no começo nenhum pintinho perto na tela
    voo = plano(1, frame, leitura, 0)
    assert (voo.lado, voo.angulo, voo.forca, voo.voar) == (1, 41, 56, True)
    # Depois do voo, sem barra à vista: a dele está atrás da minha (colado), força 5.
    curto = plano(1, frame, leitura, 1)
    assert (curto.lado, curto.angulo, curto.forca, curto.voar) == (1, 40, 5, False)
    e3 = img("e3")
    assert plano(3, e3, batalha.ler(e3), 0).voar and not plano(3, e3, batalha.ler(e3), 1).voar


def test_pintinho_quase_colado_usa_forca_5():
    frame = img("e1")
    leitura = batalha.ler(frame)
    ex, ey = leitura.eu
    leitura.inimigos = [(ex - 20, ey), (ex + 200, ey)]
    colado = plano(1, frame, leitura, 1)
    assert (colado.lado, colado.angulo, colado.forca) == (-1, 40, 5)
    leitura.inimigos = [(ex + 85, ey)]
    assert plano(1, frame, leitura, 1).forca == 10
    leitura.inimigos = []  # barra dele atrás da minha
    assert plano(1, frame, leitura, 1).forca == 5


def test_estagio_3_recua_so_no_turno_depois_do_voo_e_bate_no_lado_da_casa():
    e3 = img("e3")
    leitura = batalha.ler(e3)
    depois_do_voo, seguinte = plano(3, e3, leitura, 1), plano(3, e3, leitura, 2)
    assert (depois_do_voo.lado, depois_do_voo.angulo, depois_do_voo.forca) == (-1, 30, 20)
    assert depois_do_voo.recuar > 0 and seguinte.recuar == 0
    assert (seguinte.lado, seguinte.angulo, seguinte.forca) == (-1, 30, 20)


def test_recuar_anda_para_tras_e_vira_para_o_alvo(monkeypatch):
    frame = img("e3")
    b = batalha_de_teste(frame)
    forcas = preparar_disparo(monkeypatch, b, [True])
    assert b.disparar(batalha.ler(frame), Tiro(-1, 30, 20, "Grange", recuar=0.45))
    assert b.driver.teclas[:2] == ["right", "left"]  # para trás e de volta para a casa
    assert forcas == [20] and b.driver.teclas[-2:] == ["3", "9"]


def batalha_de_teste(frame):
    return BatalhaPintinho(Driver(frame), lambda m: None, lambda *a: None)


def test_batalha_segue_os_tiros_do_estagio_4(monkeypatch):
    vivo, morto = img("e4"), img("e4_escudo_caiu")
    b = batalha_de_teste(vivo)
    tiros = []
    monkeypatch.setattr(b, "disparar", lambda leitura, tiro: tiros.append(tiro) or True)
    for frame in (vivo, vivo, morto, morto):
        b.passo(frame)
    assert [(t.lado, t.angulo) for t in tiros] == [(-1, 20), (-1, 20), (1, 30), (1, 30)]
    assert tiros[2].forca != tiros[3].forca  # errou a gaiola: muda a força


def preparar_disparo(monkeypatch, b, angulos):
    angulos = iter(angulos)
    monkeypatch.setattr(b, "ajustar_angulo", lambda ancora, alvo: next(angulos))
    forcas = []
    monkeypatch.setattr(b, "atirar", lambda ancora, forca: forcas.append(forca))
    monkeypatch.setattr(b, "esperar_fim_da_vez", lambda: None)
    return forcas


def test_disparar_usa_os_itens_e_anda_ate_ter_o_angulo(monkeypatch):
    frame = img("e4")
    b = batalha_de_teste(frame)
    forcas = preparar_disparo(monkeypatch, b, [False, False, True])  # travou, travou, deu
    assert b.disparar(batalha.ler(frame), Tiro(-1, 20, 75, "Burrinho", andar=True))
    assert forcas == [75]
    assert b.driver.teclas[-2:] == ["3", "9"]
    passos = [t for t in b.driver.teclas if t == "right"]
    assert len(passos) == 2  # para trás (longe do Burrinho, que está à esquerda) duas vezes


def test_voar_aperta_f_e_nao_usa_itens(monkeypatch):
    frame = img("e3")
    b = batalha_de_teste(frame)
    forcas = preparar_disparo(monkeypatch, b, [True])
    assert b.disparar(batalha.ler(frame), Tiro(-1, 41, 62, "Grange", voar=True))
    assert forcas == [62] and "f" in b.driver.teclas
    assert "3" not in b.driver.teclas and "9" not in b.driver.teclas


def test_galinha_sem_angulo_exato_passa_a_vez_em_vez_de_atirar(monkeypatch):
    frame = img("e2_1de3_vento-2.5")
    b = batalha_de_teste(frame)
    forcas = preparar_disparo(monkeypatch, b, [True] * 5)
    monkeypatch.setattr(b, "angulo_exato", lambda ancora, angulo: False)
    passou = []
    monkeypatch.setattr(b, "passar_a_vez", lambda motivo: passou.append(motivo))
    b.estagio = 2
    b.passo(frame)
    assert forcas == [] and len(passou) == 1
    assert b.galinha_mirada is None and b.tiros_no_estagio == 0  # passar não conta como erro


def test_galinha_acertada_segue_para_a_proxima(monkeypatch):
    frame = img("e2_1de3_vento-2.5")
    b = batalha_de_teste(frame)
    tiros = []
    monkeypatch.setattr(b, "disparar", lambda leitura, tiro: tiros.append(tiro) or True)
    b.ler_estagio(frame, batalha.ler(frame))
    b.galinha_mirada = 0  # o tiro anterior foi na de baixo e o contador agora mostra 1/3
    b.passo(frame)
    assert tiros[0].galinha == 1 and tiros[0].angulo == 45
    assert tiros[0].forca == pytest.approx(62.9, abs=0.5)  # o usuário acertou com 63


def test_galinha_errada_mira_nela_de_novo(monkeypatch):
    # O escudo some sozinho depois (usuário, 30/10): o bot só registra o erro e tenta de novo.
    frame = img("e2_1de3_vento-2.5")
    b = batalha_de_teste(frame)
    tiros = []
    monkeypatch.setattr(b, "disparar", lambda leitura, tiro: tiros.append(tiro) or True)
    b.ler_estagio(frame, batalha.ler(frame))
    b.galinha_mirada = 1  # atirou na do meio e o contador continua em 1/3
    b.passo(frame)
    assert b.erros_galinha == 1 and tiros[0].galinha == 1


def test_minimapa_acha_galinhas_e_gaiola():
    e2, e4 = img("e2_0de3_vento-1.2"), img("e4_escudo_caiu")
    eu, vermelhos, verdes = pv.pontos_minimapa(e2, batalha.ler(e2).ancora)
    assert len(vermelhos) == 3 and verdes == []  # o botão vermelho de sair não conta
    assert [x - eu for x, _ in sorted(vermelhos, key=lambda p: -p[1])] == [114, 116, 121]
    eu, vermelhos, verdes = pv.pontos_minimapa(e4, batalha.ler(e4).ancora)
    assert vermelhos == [] and len(verdes) == 1 and verdes[0][0] - eu == 34


def test_tabela_interpola_angulos_fora_dela():
    assert pintinho.forca_tabela(30, 16) == 62
    assert pintinho.forca_tabela(50, 15) == 60
    assert pintinho.forca_tabela(40, 15) == pytest.approx(60)  # 30 e 50 dão 60 em 15
    assert pintinho.forca_tabela(55, 10) == pytest.approx(48 + (56 - 48) / 3)


def test_gaiola_pela_tabela_e_distancia():
    frame = img("e4_escudo_caiu")
    tiro = plano(4, frame, batalha.ler(frame), 4)
    # Volta 1: gaiola a 34 px do minimapa (3,9 de distância); o usuário acertou com 30°/33.
    assert tiro.angulo == 30 and tiro.forca == pytest.approx(33.5, abs=0.5)
    errou = plano(4, frame, batalha.ler(frame), 5, tiros_na_gaiola=1)
    assert errou.forca == pytest.approx(tiro.forca + 3)


def test_escolhe_pintinho_dificil_e_ok():
    frame = img("selecao")
    driver = Driver(frame)
    bot = BotPintinho(driver)
    bot.escolher_instancia(frame)
    assert len(driver.cliques) == 3
    pintinho_xy, dificil_xy, ok_xy = driver.cliques
    # Cliques do usuário no print original: Pintinho (889, 590); depois de escolhê-lo o Difícil
    # muda de lugar, então aqui (print antes da escolha) ele está no meio: (943, 742).
    assert abs(pintinho_xy[0] - (870 - ORIGEM[0])) <= 15 and abs(pintinho_xy[1] - (595 - ORIGEM[1])) <= 10
    assert abs(dificil_xy[0] - (945 - ORIGEM[0])) <= 15 and abs(dificil_xy[1] - (743 - ORIGEM[1])) <= 10
    assert ok_xy[1] > dificil_xy[1]


def test_vira_uma_carta_entre_estagios_so_quando_ela_vira():
    frame = img("cartas_estagio")
    virada = frame.copy()
    x, y = pv.cartas_fechadas(frame)[0]
    virada[y - 50:y + 50, x - 45:x + 45] = 0  # a carta clicada deixa de mostrar o verso
    bot = BotPintinho(Driver(frame))
    bot.driver.frames = [frame]  # o clique não pegou: a mesma tela
    bot.virar_carta("estagio", frame)
    assert bot.cartas_viradas == 0 and len(bot.driver.cliques) == 1
    bot.driver.frames = [virada]
    bot.virar_carta("estagio", frame)
    assert bot.cartas_viradas == 1
    bot.virar_carta("estagio", virada)
    assert len(bot.driver.cliques) == 2  # só 1 carta grátis entre estágios


def test_telas_do_pintinho_trocam_so_a_escolha_da_instancia():
    nomes = [t.nome for t in pintinho.TELAS]
    assert nomes.index("SELECAO_INSTANCIA") < nomes.index("SALA_SEM_INSTANCIA") < nomes.index("SALA_PRONTA")
    assert nomes.index("LEMBRETE") < nomes.index("SELECAO_INSTANCIA")
    assert np.all([t.cliques == [] for t in pintinho.TELAS if t.nome == "SELECAO_INSTANCIA"])


@pytest.mark.parametrize("nome, forca", [("e2_forca_56_jogo", 56.3), ("e2_forca_71_jogo", 70.8)])
def test_forca_pelo_vermelho_da_barra(nome, forca):
    # Prints do jogo de 30/10: o 1º tiro saiu com 56 (o usuário viu 55) e o log antigo dizia 62.
    frame = img(nome)
    assert pv.forca_lida(frame, batalha.ler(frame).ancora) == pytest.approx(forca, abs=0.6)


class DriverBarra(Driver):
    """A barra de força enche enquanto o Espaço está apertado; o tiro anterior fica desenhado."""

    def __init__(self, valores, depois):
        super().__init__(np.zeros((1, 1, 3), np.uint8))
        self.valores, self.depois, self.apertado = list(valores), depois, False

    def faixa(self, valor, largura=498):
        faixa = np.full((1, largura, 3), (40, 45, 46), np.uint8)
        faixa[0, 2:int(valor / 100 * (largura - 1))] = (0, 0, 133)
        return faixa

    def capturar(self, sub=None):
        if self.apertado and self.valores:
            return self.faixa(self.valores.pop(0))
        return self.faixa(self.depois if not self.apertado and not self.valores else 56)

    def segurar(self, nome):
        self.apertado = True

    def soltar(self, nome):
        self.apertado = False
        self.valores = []


def test_atirar_espera_a_barra_recomecar_e_corrige_a_antecipacao():
    # Antes: vermelho do tiro anterior em 56. A leitura só vale depois de a barra voltar a 0.
    driver = DriverBarra([56, 0, 20, 40, 58, 61.5, 64], depois=63)
    b = BatalhaPintinho(driver, lambda m: None, lambda *a: None)
    b.antecipacao = 1.0
    real = b.atirar((0, 100), 62)
    assert real == pytest.approx(63, abs=0.5)
    assert driver.valores == []  # soltou em 61,5 (62 − 1), não no 56 do tiro anterior
    assert b.antecipacao == pytest.approx(1.5, abs=0.3)  # saiu 1 a mais: solta mais cedo
