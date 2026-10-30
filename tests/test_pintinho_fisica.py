"""Modelo de trajetória (portado do ddtcurve 0.2.1) contra as tabelas da moldura e o original."""
import pytest

from src.pintinho_fisica import prever_forca

# (ângulo, vento a favor, dx, dy) -> força do ddtcurve.predict (scipy), calculada em 30/10.
ORIGINAL = [((30, 0, 16, 0), 64.30), ((45, 0, 13.29, 3.70), 64.32),
            ((55, -2.4, 13.99, 6.13), 73.35), ((30, -1.3, 13.18, 0.81), 61.57)]


@pytest.mark.parametrize("args, forca", ORIGINAL)
def test_igual_ao_ddtcurve(args, forca):
    assert prever_forca(*args) == pytest.approx(forca, abs=0.02)


@pytest.mark.parametrize("angulo, tabela", [
    (20, {5: 36, 10: 54, 15: 69, 20: 80}), (30, {5: 32, 10: 47, 15: 60, 20: 72}),
    (50, {5: 32, 10: 48, 15: 60, 20: 72}), (65, {5: 37, 10: 56, 15: 70}),
])
def test_perto_das_tabelas_da_moldura(angulo, tabela):
    for distancia, forca in tabela.items():
        assert prever_forca(angulo, 0, distancia, 0) == pytest.approx(forca, abs=3.5)


def test_vento_e_altura_mudam_a_forca_no_sentido_certo():
    base = prever_forca(45, 0, 13, 3)
    assert prever_forca(45, -3, 13, 3) > base > prever_forca(45, 3, 13, 3)  # contra pede mais
    assert prever_forca(45, 0, 13, 6) > base                                # mais alto pede mais
    assert prever_forca(45, 0, 0, 3) is None and prever_forca(10, -9, 19, 8) is None
