"""Triângulo da vez: o ícone azul do item no painel não pode ser confundido com ele.

As imagens em tests/imagens/ são prints do registro de tiro da run de 29/10 (JPEG
anotado pelo bot: círculos e seta desenhados por cima, não fazem parte do jogo).
"""
from pathlib import Path

import cv2

from src.batalha import ler

IMAGENS = Path(__file__).resolve().parent / "imagens"


def test_icone_do_item_no_painel_nao_vira_eu():
    # Turno 4: o triângulo de verdade estava na fase apagada da animação e o ícone azul
    # do item (1200, 726), abaixo da linha das barras, foi lido como triângulo.
    leitura = ler(cv2.imread(str(IMAGENS / "tiro_triangulo_falso_hud.jpg")))
    assert leitura.eu is not None and abs(leitura.eu[0] - 961) <= 3
    assert leitura.aliados == []
    assert not leitura.pelo_triangulo


def test_triangulo_real_acima_da_barra_continua_valendo():
    leitura = ler(cv2.imread(str(IMAGENS / "tiro_triangulo_real.jpg")))
    assert abs(leitura.eu[0] - 962) <= 3
    assert leitura.pelo_triangulo
