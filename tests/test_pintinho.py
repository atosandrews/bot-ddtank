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
    assert (voo.lado, voo.angulo, voo.forca, voo.voar) == (1, 40, 45, True)
    # Depois do voo, sem barra à vista: a dele está atrás da minha (colado), força 5.
    curto = plano(1, frame, leitura, 1)
    assert (curto.lado, curto.angulo, curto.forca, curto.voar) == (1, 40, 5, False)
    e3 = img("e3")
    assert plano(3, e3, batalha.ler(e3), 0).voar and not plano(3, e3, batalha.ler(e3), 1).voar


def test_voo_vai_sempre_para_o_centro_do_mapa(monkeypatch):
    frame = img("e1")
    leitura = batalha.ler(frame)
    leitura.inimigos = []
    monkeypatch.setattr(pv, "lado_dos_inimigos", lambda f, a: 1)  # pintinhos à direita...
    monkeypatch.setattr(pv, "lado_do_mapa", lambda f, a: -1)  # ...mas nasceu na metade direita
    voo = plano(1, frame, leitura, 0)
    assert voo.voar and voo.lado == -1 and "centro do mapa" in voo.motivo
    e3 = img("e3")
    assert plano(3, e3, batalha.ler(e3), 0).lado == -1
    monkeypatch.setattr(pv, "lado_do_mapa", lambda f, a: None)
    leitura.eu = (leitura.ancora[0] + 667, leitura.eu[1])  # x do 5º teste: lado direito da tela
    assert plano(1, frame, leitura, 0).lado == -1


@pytest.mark.parametrize("nome, lado", [("e1", 1), ("e3", 1)])
def test_lado_do_centro_do_mapa_nos_prints(nome, lado):
    # e1: nasceu à esquerda do mapa (voou para a direita na gravação). e3 é o turno depois do voo
    # do estágio 3: o personagem já está na metade esquerda (no começo nascia na direita).
    frame = img(nome)
    assert pv.lado_do_mapa(frame, batalha.ler(frame).ancora) == lado


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
    monkeypatch.setattr(pv, "pontos_minimapa", lambda frame, ancora: None)  # sem minimapa
    assert b.disparar(batalha.ler(frame), Tiro(-1, 30, 20, "Grange", recuar=7))
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


def test_disparar_usa_os_itens_e_anda_para_tras_ate_ter_o_angulo(monkeypatch):
    frame = img("e4")
    b = batalha_de_teste(frame)
    forcas = preparar_disparo(monkeypatch, b, [False, True])  # travou aqui; depois deu o exato
    desce = iter([False, True])  # o 20 aparece no 2º passo para trás
    monkeypatch.setattr(b, "angulo_desce_ate", lambda ancora, angulo: next(desce))
    assert b.disparar(batalha.ler(frame), Tiro(-1, 20, 75, "Burrinho", andar=True))
    assert forcas == [75]
    assert b.driver.teclas[-2:] == ["3", "9"]
    assert b.driver.teclas.count("right") == 2  # para trás (o Burrinho está à esquerda)


def test_voar_aperta_f_e_nao_usa_itens(monkeypatch):
    frame = img("e3")
    b = batalha_de_teste(frame)
    forcas = preparar_disparo(monkeypatch, b, [True])
    assert b.disparar(batalha.ler(frame), Tiro(-1, 41, 62, "Grange", voar=True))
    assert forcas == [62] and "f" in b.driver.teclas
    assert "3" not in b.driver.teclas and "9" not in b.driver.teclas


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
def test_marca_do_tiro_anterior(nome, forca):
    # Prints do jogo de 30/10, no começo da vez: o 1º tiro saiu com 56 (o usuário viu 55) e o
    # log antigo dizia 62. A barra ao vivo já está escura (zera quando o Espaço é apertado).
    frame = img(nome)
    ancora = batalha.ler(frame).ancora
    assert pv.ultimo_tiro(frame, ancora) == pytest.approx(forca, abs=0.6)
    assert pv.forca_lida(frame, ancora) < 3


@pytest.mark.parametrize("cor", [(82, 164, 226), (91, 196, 230)])  # tiro normal e voo (amarelo)
def test_barra_ao_vivo_em_qualquer_cor(cor):
    faixa = np.full((1, 498, 3), (28, 33, 34), np.uint8)
    faixa[0, 3:249] = cor
    assert pv.forca_na_faixa(faixa) == pytest.approx(50, abs=0.5)


class DriverBarra(Driver):
    """A barra de força enche enquanto o Espaço está apertado; o tiro anterior fica desenhado."""

    def __init__(self, valores, depois):
        super().__init__(np.zeros((1, 1, 3), np.uint8))
        self.valores, self.depois, self.apertado = list(valores), depois, False

    def faixa(self, valor, largura=498):
        faixa = np.full((1, largura, 3), (28, 33, 34), np.uint8)
        faixa[0, 2:int(valor / 100 * (largura - 1))] = (82, 164, 226)
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


def test_atirar_espera_a_barra_recomecar_e_preve_pela_velocidade(monkeypatch):
    # Antes: o tiro anterior desenhado em 56. A leitura só vale depois de a barra voltar a 0.
    relogio = iter(i * 0.05 for i in range(1000))  # um print a cada 50 ms
    monkeypatch.setattr(pintinho.time, "time", lambda: next(relogio))
    driver = DriverBarra([56, 0, 20, 40, 52, 58, 61.5, 64], depois=63)
    linhas = []
    b = BatalhaPintinho(driver, linhas.append, lambda *a: None)
    b.atraso = 0.03
    real = b.atirar((0, 100), 62)
    assert real == pytest.approx(63, abs=0.5)
    soltou = float(linhas[-1].split("lida ao soltar ")[1].split(",")[0])
    assert 52 <= soltou < 61.5  # previu pela velocidade; não soltou no 56 do tiro anterior
    assert b.atraso > 0.03  # saiu 1 a mais: solta mais cedo da próxima vez


def test_recuo_anda_ate_o_minimapa_mostrar_os_pixels(monkeypatch):
    frame = img("e3")
    b = batalha_de_teste(frame)
    posicoes = iter([100, 101, 103, 106, 108, 120])  # x meu no minimapa a cada print
    monkeypatch.setattr(pv, "pontos_minimapa", lambda f, a: (next(posicoes), [], []))
    b.recuar(batalha.ler(frame).ancora, -1, 7)  # alvo à esquerda: recua para a direita
    assert b.driver.teclas == ["right"] * 4  # 101, 103, 106, 108: parou ao passar de 7


@pytest.mark.parametrize("nome", ["e1", "e3", "e4", "e2_0de3_vento-1.2"])
def test_relogio_le_igual_ou_melhor_que_o_do_bot(nome):
    frame = img(nome)
    ancora = batalha.ler(frame).ancora
    bot = batalha.ler_numero(frame, ancora)
    novo = pintinho.ler_relogio(frame, ancora)
    assert novo[1]
    if bot[1]:
        assert novo == bot


def test_28_conta_um_toque_a_partir_do_29_e_aprende_o_8(monkeypatch, tmp_path):
    # Como era antes de o bot aprender o 8 no jogo: só os dígitos do bot e o 6 e o 9.
    import shutil
    for d in ("6", "9"):
        shutil.copy(PASTA.parent.parent.parent / "img" / "pintinho" / "digitos" / f"{d}.png", tmp_path)
    monkeypatch.setattr(pintinho, "PASTA_DIGITOS", tmp_path)
    monkeypatch.setattr(pintinho.batalha, "_digitos", {})
    assert not pintinho.angulo_legivel(28) and pintinho.angulo_legivel(29)
    frame = img("e2_1de3_vento-2.5")
    b = batalha_de_teste(frame)
    alvos = []
    monkeypatch.setattr(b, "_ajustar_com_relogio", lambda ancora, alvo: alvos.append(alvo) or True)
    mascaras = iter([np.zeros((28, 50), np.uint8), np.ones((28, 50), np.uint8) * 255])
    monkeypatch.setattr(pintinho.batalha, "mascara_numero", lambda f, a: next(mascaras, np.ones((28, 50), np.uint8) * 255))
    monkeypatch.setattr(pintinho, "ler_relogio", lambda f, a: (25, False))
    aprendidos = []
    monkeypatch.setattr(b, "aprender_digito", lambda ancora, alvo: aprendidos.append(alvo))
    assert b.ajustar_angulo(batalha.ler(frame).ancora, 28)
    assert alvos == [29] and b.driver.teclas == ["down"] and aprendidos == [28]



def test_relogio_le_28_laranja(monkeypatch):
    # 7º teste (30/10): no pixel de trás o 28 laranja ficou ilegível e o bot passou a vez.
    monkeypatch.setattr(pintinho.batalha, "_digitos", {})  # recarrega com o 8 aprendido no jogo
    frame = img("e2_relogio_28_laranja")
    assert pintinho.ler_relogio(frame, batalha.ler(frame).ancora) == (28, True)


def test_andar_ate_o_angulo_respeita_o_tempo_da_vez(monkeypatch):
    frame = img("e4")
    b = batalha_de_teste(frame)
    relogio = iter([0.0, 0.0, 10.0, 10.0, 10.0])  # já passou de 7 s da vez no 2º passo
    monkeypatch.setattr(pintinho.time, "time", lambda: next(relogio, 99.0))
    b.inicio_vez = 0.0
    monkeypatch.setattr(b, "angulo_desce_ate", lambda ancora, alvo: False)
    assert not b.andar_ate_angulo(batalha.ler(frame), -1, 20)
    assert b.driver.teclas.count("right") == 1  # um passo e parou: passou de 7 s da vez


@pytest.mark.parametrize("nome, mortas, angulo, forca", [
    # Volta 1 da gravação (vento contra): o usuário matou com 62 / 63 / 71.
    ("e2_0de3_vento-1.2", 0, 30, 62.0 + 1.8),
    ("e2_1de3_vento-2.5", 1, 45, 66.2 - 1.1),
    # Volta 2 (nasceu 9 px mais perto, vento a favor 2,7): o usuário matou a de cima com 68.
    ("e2_vento+2.7", 2, 55, 66.9 + 1.5),
])
def test_estagio_2_calcula_a_forca_pela_distancia_altura_e_vento(nome, mortas, angulo, forca):
    frame = img(nome)
    tiro = plano(2, frame, batalha.ler(frame), mortas, mortas=mortas)
    assert (tiro.lado, tiro.angulo, tiro.galinha) == (1, angulo, mortas)
    assert tiro.forca == pytest.approx(forca, abs=0.2) and "pelo minimapa" in tiro.motivo
    dx, dy, a_favor = tiro.calculo
    assert 12 < dx < 14.2 and dy >= 0.5


def test_estagio_2_sem_minimapa_usa_o_lugar_da_gravacao(monkeypatch):
    frame = img("e2_0de3_vento-1.2")
    monkeypatch.setattr(pv, "posicao_no_minimapa", lambda f, a: None)
    tiro = plano(2, frame, batalha.ler(frame), 0, mortas=0, vento=0.0)
    assert "lugar da gravação" in tiro.motivo
    assert tiro.calculo[0] == pytest.approx(114 * pv.DISTANCIA_POR_PX_MINIMAPA)


def test_galinha_com_angulo_travado_recalcula_a_forca(monkeypatch):
    frame = img("e2_0de3_vento-1.2")
    b = batalha_de_teste(frame)
    forcas = preparar_disparo(monkeypatch, b, [False] * 5)  # o 30 não dá: o relógio fica em 35
    monkeypatch.setattr(pintinho, "ler_relogio", lambda f, a: (35, True))
    tiro = plano(2, frame, batalha.ler(frame), 0, mortas=0, vento=0.0)
    assert b.disparar(batalha.ler(frame), tiro)
    dx, dy, a_favor = tiro.calculo
    esperado = pintinho.prever_forca(35, a_favor, dx, dy) + pintinho.GALINHA_AJUSTE[0]
    assert forcas == [pytest.approx(esperado)]


def test_galinha_sem_ler_o_relogio_passa_a_vez(monkeypatch):
    frame = img("e2_0de3_vento-1.2")
    b = batalha_de_teste(frame)
    forcas = preparar_disparo(monkeypatch, b, [True] * 5)
    monkeypatch.setattr(pintinho, "ler_relogio", lambda f, a: (None, False))
    passou = []
    monkeypatch.setattr(b, "passar_a_vez", lambda motivo: passou.append(motivo))
    b.estagio = 2
    b.passo(frame)
    assert forcas == [] and len(passou) == 1
    assert b.galinha_mirada is None and b.tiros_no_estagio == 0  # passar não conta como tiro


def test_galinha_acertada_segue_para_a_proxima(monkeypatch):
    frame = img("e2_1de3_vento-2.5")
    b = batalha_de_teste(frame)
    tiros = []
    monkeypatch.setattr(b, "disparar", lambda leitura, tiro: tiros.append(tiro) or True)
    b.ler_estagio(frame, batalha.ler(frame))
    b.galinha_mirada = 0  # o tiro anterior foi na de baixo e o contador agora mostra 1/3
    b.passo(frame)
    assert tiros[0].galinha == 1 and tiros[0].angulo == 45

