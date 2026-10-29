"""Gerador de captchas sintéticos e métricas do avaliador, sem carregar modelos ONNX."""
import numpy as np
import pytest

from src.verificacao_ocr import Candidato
from tools.avaliar_ocr import avaliar, classificar_erro, top1
from tools.gerar_captchas import CASOS_ADVERSARIAIS, NIVEIS, gerar, gerar_lote


@pytest.mark.parametrize("nivel", NIVEIS)
def test_gerador_formato_e_rotulo(nivel):
    for i in range(3):
        c = gerar(7, nivel, i)
        assert c.imagem.shape == (68, 164, 3)
        assert c.imagem.dtype == np.uint8
        assert len(c.rotulo) == 4 and c.rotulo.isdigit()
        assert len(c.cores) == 4
        # fundo claro (~243) como o recorte real; dígitos e ruído escurecem o resto
        assert np.percentile(c.imagem, 75) > 200


def test_gerador_deterministico_pela_semente():
    a, b = gerar(11, "medio", 3), gerar(11, "medio", 3)
    assert a.rotulo == b.rotulo and np.array_equal(a.imagem, b.imagem)
    outro = gerar(12, "medio", 3)
    assert not np.array_equal(a.imagem, outro.imagem)


def test_amostra_independe_do_tamanho_do_lote():
    lote = gerar_lote(5, "dificil", 4)
    assert np.array_equal(lote[2].imagem, gerar(5, "dificil", 2).imagem)


def test_rotulo_fixo_e_casos_adversariais():
    assert gerar(1, "facil", 0, rotulo="0007").rotulo == "0007"
    casos = {gerar(1, "adversarial", i).caso for i in range(len(CASOS_ADVERSARIAIS))}
    assert casos == set(CASOS_ADVERSARIAIS)
    assert gerar(1, "adversarial", 0, caso="mesma_cor_toda").cores == [0, 0, 0, 0]
    sep = gerar(1, "adversarial", 0, caso="mesma_cor_separada").cores
    assert sep in ([0, 1, 0, 2], [0, 1, 2, 1])
    with pytest.raises(ValueError):
        gerar(1, "facil", 0, rotulo="12a4")
    with pytest.raises(ValueError):
        gerar(1, "impossivel", 0)


def test_classificar_erro():
    assert classificar_erro("6723", "7623") == "ordem_trocada"
    assert classificar_erro("6723", "6728") == "um_digito_confundido"
    assert classificar_erro("1234", "2345") == "digito_faltando_e_extra"
    assert classificar_erro("1234", "5678") == "varios_digitos"


def test_top1_respeita_limiar_e_variante():
    leituras = [{"texto": "1111", "confianca": .99, "modelo": "beta", "variante": "original"},
                {"texto": "2222", "confianca": .95, "modelo": "beta", "variante": "cores"},
                {"texto": "333", "confianca": 1.0, "modelo": "beta", "variante": "cores"}]
    assert top1(leituras, .9)["texto"] == "1111"
    assert top1(leituras, .9, "cores")["texto"] == "2222"
    assert top1(leituras, .995) is None


class OcrFalso:
    """Lê sempre a mesma coisa; imita reconhecer() e ultimas_leituras do OcrLocal."""

    def __init__(self, texto, confianca):
        self.texto, self.confianca = texto, confianca

    def reconhecer(self, imagem, limiar=0.90):
        self.ultimas_leituras = [{"texto": self.texto, "confianca": self.confianca,
                                  "modelo": "beta", "variante": "original"}]
        if self.confianca >= limiar and len(self.texto) == 4:
            return [Candidato(self.texto, self.confianca, "beta", "original")]
        return []


def test_avaliar_conta_certo_errado_e_sem_candidato():
    img = np.zeros((68, 164, 3), np.uint8)
    amostras = [("x", "x/0", "1234", img, ""), ("x", "x/1", "1235", img, ""),
                ("x", "x/2", "1243", img, "")]
    r = avaliar(OcrFalso("1234", .97), amostras, .9)["x"]
    geral = r["variantes"]["geral"]
    assert (geral["certo"], geral["errado"], geral["sem_candidato"]) == (1, 2, 0)
    assert r["erros_por_categoria"] == {"um_digito_confundido": 1, "ordem_trocada": 1}
    assert r["variantes"]["cores"]["sem_candidato"] == 3
    # com limiar acima da confiança, nada é enviado
    assert r["varredura_limiar_geral"]["0.98"]["sem_candidato"] == 3


class OcrComVeto(OcrFalso):
    """Leitura elegível, mas o bot não envia nada (como o consenso de `decidir`)."""

    def reconhecer(self, imagem, limiar=0.90):
        super().reconhecer(imagem, limiar)
        return []


def test_geral_e_exatamente_o_que_o_bot_envia():
    img = np.zeros((68, 164, 3), np.uint8)
    r = avaliar(OcrComVeto("1234", .99), [("x", "x/0", "1235", img, "")], .9)["x"]
    assert r["variantes"]["geral"]["sem_candidato"] == 1  # não usa o maior score
    assert r["variantes"]["original"]["errado"] == 1      # diagnóstico por variante
    assert r["exemplos_erro"] == []


def test_varredura_usa_o_decisor_informado():
    img = np.zeros((68, 164, 3), np.uint8)
    nunca = lambda leituras, limiar: []  # noqa: E731
    r = avaliar(OcrFalso("1234", .99), [("x", "x/0", "1234", img, "")], .9, decisor=nunca)["x"]
    assert r["variantes"]["geral"]["certo"] == 1
    assert all(v["sem_candidato"] == 1 for v in r["varredura_limiar_geral"].values())
