import importlib.util
from pathlib import Path

import cv2
import pytest

RAIZ = Path(__file__).resolve().parents[1]
_spec = importlib.util.spec_from_file_location("gravar_sessao", RAIZ / "tools/gravar_sessao.py")
gravar_sessao = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(gravar_sessao)


@pytest.fixture
def frame():
    imagem = cv2.imread(str(RAIZ / "docs/especificacao/imagens/24_formigas_minha_vez_janela_real.png"))
    assert imagem is not None
    return imagem


def test_mudou_ignora_o_mesmo_print_e_ve_tela_diferente(frame):
    assert gravar_sessao.mudou(None, frame)
    assert not gravar_sessao.mudou(frame, frame.copy())
    assert gravar_sessao.mudou(frame, 255 - frame)


def test_ler_tiro_le_angulo_e_lado_no_print_da_batalha(frame):
    from src import batalha
    ancora = batalha.ler(frame).ancora
    barra = gravar_sessao.recorte_barra(frame, ancora)
    tiro = gravar_sessao.ler_tiro(frame, barra, 1.234)
    assert tiro["segurou_s"] == 1.23
    assert tiro["angulo"] is not None
    assert tiro["forca"] == 0  # mesma barra da referência: nada encheu
    assert "ângulo" in gravar_sessao.descrever_tiro(tiro)


def test_ler_tiro_fora_da_batalha_nao_quebra(frame):
    tiro = gravar_sessao.ler_tiro(frame[:50, :50], None, 0.5)
    assert tiro["angulo"] is None and tiro["forca"] is None


def test_resumo_do_turno_soma_passos_itens_e_ajuste_de_angulo():
    turno = gravar_sessao.Turno(0.0, teclas=[("3", 0.1), ("9", 0.1), ("direita", 0.4),
                                             ("direita", 0.3), ("cima", 0.05), ("cima", 0.05),
                                             ("espaco", 1.2)],
                                tiros=[{"segurou_s": 1.2, "angulo": 30, "lado": "direita", "forca": 80}])
    resumo = turno.resumo()
    assert "itens 3 9" in resumo
    assert "andou direita 0.7s" in resumo
    assert "ângulo ↑2 ↓0" in resumo
    assert "tiro ângulo 30°, força 80, direita" in resumo
