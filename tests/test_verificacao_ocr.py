from pathlib import Path

import cv2
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


class ModeloPorPosicao(Modelo):
    """Lê a posição dos pixels escuros: só a variante por cores devolve os dígitos certos."""
    def classification(self, png, probability=False):
        imagem = cv2.imdecode(np.frombuffer(png, np.uint8), cv2.IMREAD_GRAYSCALE)
        _, xs = np.nonzero(imagem < 128)
        if xs.size == 0:
            texto = ""
        elif xs.max() - xs.min() > imagem.shape[1] * .6:
            texto = "99"  # várias camadas juntas viram leitura truncada
        else:
            texto = "12" if xs.mean() < imagem.shape[1] / 2 else "34"
        return {"text": texto, "confidence": self.confianca} if probability else texto


def captcha_sintetico():
    """Dígitos azuis à esquerda e vermelhos à direita, com linhas claras de ruído."""
    imagem = np.full((68, 164, 3), 245, np.uint8)
    cv2.line(imagem, (0, 10), (163, 60), (200, 190, 210), 2)
    cv2.line(imagem, (0, 55), (163, 5), (190, 210, 200), 2)
    cv2.rectangle(imagem, (20, 15), (55, 55), (160, 40, 30), -1)
    cv2.rectangle(imagem, (100, 15), (140, 55), (30, 30, 150), -1)
    return imagem


def test_camadas_de_cor_sao_lidas_separadas_e_ordenadas_pela_posicao():
    ocr = OcrLocal(ModeloPorPosicao)
    candidatos = ocr.reconhecer(captcha_sintetico())
    assert [(c.texto, c.variante) for c in candidatos] == [("1234", "cores")]
    assert candidatos[0].confianca == .95


def test_ruido_claro_nao_forma_camada():
    imagem = np.full((68, 164, 3), 245, np.uint8)
    cv2.line(imagem, (0, 10), (163, 60), (200, 190, 210), 2)
    assert OcrLocal.camadas_por_cor(imagem) == []


def test_camada_sem_leitura_descarta_o_candidato_por_cores():
    class ModeloFalhaNaDireita(ModeloPorPosicao):
        def classification(self, png, probability=False):
            r = super().classification(png, probability=True)
            if r["text"] == "34":
                r["text"] = ""
            return r if probability else r["text"]
    assert OcrLocal(ModeloFalhaNaDireita).reconhecer(captcha_sintetico()) == []


def test_camada_com_digitos_dos_dois_lados_de_outra_nao_tem_ordem_e_e_descartada():
    # Azul à esquerda e à direita do vermelho: juntar por uma única posição inverteria dígitos.
    imagem = np.full((68, 164, 3), 245, np.uint8)
    cv2.rectangle(imagem, (10, 15), (35, 55), (160, 40, 30), -1)
    cv2.rectangle(imagem, (70, 15), (95, 55), (30, 30, 150), -1)
    cv2.rectangle(imagem, (130, 15), (155, 55), (160, 40, 30), -1)
    assert OcrLocal(ModeloPorPosicao).reconhecer(imagem) == []


def test_modelos_reais_leem_o_captcha_da_especificacao():
    pytest.importorskip("ddddocr")
    from src.verificacao_visao import VisaoVerificacao
    raiz = Path(__file__).resolve().parents[1]
    frame = cv2.imread(str(raiz / "docs/especificacao/imagens/38_codigo_verificacao.png"))
    candidatos = OcrLocal().reconhecer(VisaoVerificacao().analisar(frame).codigo)
    assert candidatos and candidatos[0].texto == "6723"
    assert {c.variante for c in candidatos} == {"cores"}
