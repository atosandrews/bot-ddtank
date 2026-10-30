from pathlib import Path

import cv2
import numpy as np
import pytest

from src.verificacao_visao import QuadroInvalido, VisaoVerificacao

RAIZ = Path(__file__).resolve().parents[1]


@pytest.fixture
def frame():
    imagem = cv2.imread(str(RAIZ / "docs/especificacao/imagens/38_codigo_verificacao.png"))
    assert imagem is not None
    return imagem


def test_perfil_localiza_codigo_dez_teclas_e_controles(frame):
    q = VisaoVerificacao().analisar(frame)
    assert q.origem == (0, 0)
    assert q.codigo.shape == (68, 164, 3)
    assert q.ocupados == 0
    assert q.teclas["0"] == (431, 83)
    assert q.teclas["7"] == (398, 239)
    assert set(q.teclas) == set("0123456789")
    assert q.confirmar == (129, 296)
    assert q.resetar == (387, 296)


def test_popup_deslocado_mantem_recortes_e_transforma_coordenadas(frame):
    v = VisaoVerificacao()
    original = v.analisar(frame)
    grande = np.zeros((450, 700, 3), dtype=np.uint8)
    grande[55:369, 70:587] = frame
    q = v.analisar(grande)
    assert q.origem == (70, 55)
    assert q.identidade == original.identidade
    assert q.teclas["0"] == (501, 138)


def test_numeros_sao_localizados_na_imagem_e_nao_por_posicao_fixa(frame):
    v = VisaoVerificacao()
    for digito in "01":
        x, y, w, h = v.perfil["templates"][digito]
        frame[y:y+h, x:x+w] = (130, 170, 190)
    for digito, (x, y) in {"0": (364, 43), "1": (419, 75)}.items():
        tpl = v.templates[digito]
        h, w = tpl.shape[:2]
        frame[y:y+h, x:x+w] = tpl
    q = v.analisar(frame)
    assert q.teclas["0"] == (376, 51)
    assert q.teclas["1"] == (433, 83)


def test_destino_duplicado_cai_na_roda_e_usa_o_botao_de_verdade(frame):
    # Um "0" solto no meio do teclado deixa o recorte exato ambíguo; a roda só considera
    # as posições do círculo, então o 0 continua sendo o botão real.
    v = VisaoVerificacao()
    tpl = v.templates["0"]
    frame[110:126, 350:375] = tpl
    q = v.analisar(frame)
    assert abs(q.teclas["0"][0] - 431) <= 4 and abs(q.teclas["0"][1] - 83) <= 4


def test_teclado_sem_botoes_nao_e_clicado(frame):
    v = VisaoVerificacao()
    kx, ky, kw, kh = v.perfil["keyboard"]
    frame[ky + 12:ky + kh - 12, kx + 12:kx + kw - 12] = (130, 170, 190)
    with pytest.raises(QuadroInvalido):
        v.analisar(frame)


def test_popup_cortado_e_rejeitado(frame):
    with pytest.raises(QuadroInvalido, match="cortado"):
        VisaoVerificacao().analisar(frame[:, :400])


def test_sem_titulo_nao_e_captcha():
    assert VisaoVerificacao().analisar(np.zeros((400, 600, 3), dtype=np.uint8)) is None


def test_conta_casas_preenchidas_e_rejeita_buraco(frame):
    v = VisaoVerificacao()
    cv2.putText(frame, "0", (39, 174), cv2.FONT_HERSHEY_SIMPLEX, .55, (0, 0, 0), 1)
    assert v.analisar(frame).ocupados == 1
    cv2.putText(frame, "1", (120, 174), cv2.FONT_HERSHEY_SIMPLEX, .55, (0, 0, 0), 1)
    with pytest.raises(QuadroInvalido, match="inconsistentes"):
        v.analisar(frame)


RODA_GIRADA = Path(__file__).resolve().parent / "imagens" / "captcha_roda_girada.webp"
# Print do usuário (29/10): a roda girou; 3 e 4 embaixo e grandes, 9 e 8 em cima e pequenos.
CENTROS_RODA_GIRADA = {"9": (994, 373), "8": (1056, 374), "0": (943, 409), "7": (1105, 412),
                       "1": (923, 467), "6": (1123, 471), "2": (941, 526), "5": (1103, 529),
                       "3": (990, 564), "4": (1052, 565)}


def test_roda_girada_com_botoes_de_outro_tamanho_e_reconhecida():
    q = VisaoVerificacao().analisar(cv2.imread(str(RODA_GIRADA)))
    for digito, (x, y) in CENTROS_RODA_GIRADA.items():
        assert abs(q.teclas[digito][0] - x) <= 4 and abs(q.teclas[digito][1] - y) <= 4, digito


def test_roda_concorda_com_o_recorte_exato_na_referencia(frame):
    v = VisaoVerificacao()
    exato = v.analisar(frame).teclas
    kx, ky, kw, kh = v.perfil["keyboard"]
    roda = v.teclas_pela_roda(frame[ky:ky + kh, kx:kx + kw], (kx, ky))
    assert all(np.hypot(*np.subtract(exato[d], roda[d])) <= 4 for d in "0123456789")


def test_segundo_captcha_real_com_roda_girada():
    # Print do usuário (29/10, 2º): 5 e 4 em cima, 9 e 0 embaixo e grandes.
    esperado = {"7": (960, 511), "6": (986, 455), "5": (1039, 425), "4": (1101, 432),
                "3": (1146, 474), "2": (1158, 535), "1": (1132, 591), "0": (1079, 621),
                "9": (1017, 614), "8": (972, 572)}
    q = VisaoVerificacao().analisar(cv2.imread(str(RODA_GIRADA.with_name("captcha_roda_girada_2.webp"))))
    for digito, (x, y) in esperado.items():
        assert abs(q.teclas[digito][0] - x) <= 4 and abs(q.teclas[digito][1] - y) <= 4, digito
    assert q.repouso is not None and q.repouso[1] < q.centro_roda[1] - 100  # acima do teclado


def test_aviso_clique_no_jogo_sobre_o_captcha_da_ponto_fora_do_teclado():
    # Quadro do vídeo da run de 29/10: "Clique no jogo" escurece e cobre o teclado.
    frame = cv2.imread(str(RODA_GIRADA.with_name("captcha_sem_foco.jpg")))
    v = VisaoVerificacao()
    ponto = v.ponto_para_foco(frame)
    assert ponto is not None
    ax, ay = v.ancora(frame)
    ox, oy = ax - v.perfil["anchor"][0], ay - v.perfil["anchor"][1]
    kx, ky, kw, kh = v.perfil["keyboard"]
    cx, cy, cw, ch = v.perfil["controls"]
    fora = lambda x, y, w, h: not (ox + x <= ponto[0] <= ox + x + w and oy + y <= ponto[1] <= oy + y + h)
    assert fora(kx, ky, kw, kh) and fora(cx, cy, cw, ch)
    assert ponto[1] < oy + 25  # barra do título


def test_sem_aviso_nao_ha_clique_de_foco():
    frame = cv2.imread(str(RODA_GIRADA.with_name("captcha_roda_girada_2.webp")))
    assert VisaoVerificacao().ponto_para_foco(frame) is None


def test_conferir_com_mouse_em_cima_mira_o_centro_do_botao_e_recusa_lugar_vazio():
    frame = cv2.imread(str(RODA_GIRADA.with_name("captcha_roda_girada_2.webp")))
    v = VisaoVerificacao()
    q = v.analisar(frame)
    x, y = q.teclas["5"]
    ok, ponto = v.conferir(frame, q.origem, "5", (x + 6, y + 3))  # mira um pouco torta
    assert ok and np.hypot(*np.subtract(ponto, q.teclas["5"])) <= 4
    ok, _ = v.conferir(frame, q.origem, "5", q.centro_roda)  # meio da roda: sem botão
    assert not ok


def test_botao_grande_partido_em_pedacos_nao_derruba_a_roda():
    # Quadro do vídeo de 30/10 (conta2): pedaços dos botões de baixo davam "fora do círculo".
    q = VisaoVerificacao().analisar(cv2.imread(str(RODA_GIRADA.with_name("captcha_botao_partido.png"))))
    esperado = {"8": (267, 121), "7": (305, 72), "6": (364, 54), "5": (422, 74), "4": (458, 125),
                "3": (457, 187), "2": (419, 236), "1": (360, 254), "0": (302, 234), "9": (266, 183)}
    for d, (x, y) in esperado.items():
        assert abs(q.teclas[d][0] - x) <= 4 and abs(q.teclas[d][1] - y) <= 4, d


def test_mouse_espera_perto_dos_numeros_sem_ficar_em_cima():
    frame = cv2.imread(str(RODA_GIRADA.with_name("captcha_botao_partido.png")))
    q = VisaoVerificacao().analisar(frame)
    assert min(np.hypot(*np.subtract(q.repouso, p)) for p in q.teclas.values()) >= 30
    assert np.hypot(*np.subtract(q.repouso, q.centro_roda)) < 180  # dentro do painel do teclado
