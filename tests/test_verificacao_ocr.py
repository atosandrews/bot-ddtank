import numpy as np
import pytest

from src.verificacao_ocr import ErroInfraestrutura, OcrLocal


class Modelo:
    def __init__(self, *, show_ad, use_gpu, beta):
        assert not show_ad and not use_gpu
        self.beta = beta
        self.texto = "0011"
        self.confianca = .95 if beta else .94

    def set_ranges(self, caracteres):
        assert caracteres == "0123456789"

    def classification(self, png, probability=False):
        assert png.startswith(b"\x89PNG")
        return {"text": self.texto, "confidence": self.confianca} if probability else self.texto


def test_preserva_zero_inicial_repetidos_e_deduplica_variantes():
    ocr = OcrLocal(Modelo)
    candidatos = ocr.reconhecer(np.ones((20, 60, 3), np.uint8))
    assert len(candidatos) == 1
    assert candidatos[0].texto == "0011"
    assert candidatos[0].modelo == "beta"


@pytest.mark.parametrize("texto,score", [("", .99), ("12", .99), ("12345", .99),
    ("O123", .99), (" 1234", .99), ("１２３４", .99), ("1234", .89),
    ("1234", float("nan")), ("1234", 1.1)])
def test_rejeita_texto_invalido_e_score_insuficiente(texto, score):
    ocr = OcrLocal(Modelo)
    for _, modelo in ocr.modelos:
        modelo.texto, modelo.confianca = texto, score
    assert ocr.reconhecer(np.ones((20, 60, 3), np.uint8)) == []


def test_modelo_ausente_e_erro_de_infraestrutura():
    def falhar(**kwargs):
        raise FileNotFoundError("modelo")
    with pytest.raises(ErroInfraestrutura, match="OCR indisponível"):
        OcrLocal(falhar)


def test_imagem_vazia_nao_e_enviada_ao_modelo():
    assert OcrLocal(Modelo).reconhecer(np.zeros((0, 0, 3), np.uint8)) == []
