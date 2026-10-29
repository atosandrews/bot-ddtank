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


def test_destino_duplicado_e_rejeitado(frame):
    v = VisaoVerificacao()
    tpl = v.templates["0"]
    frame[110:126, 350:375] = tpl
    with pytest.raises(QuadroInvalido, match="ambíguo"):
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
