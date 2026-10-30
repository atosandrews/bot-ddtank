from pathlib import Path

import cv2
import numpy as np
import pytest

from src.verificacao_ocr import ErroInfraestrutura, OcrLocal, _intercaladas, decidir


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


def test_camadas_intercaladas_sao_descartadas_mesmo_com_pedaco_maior_fora_do_intervalo():
    # Azul em 10-40 (maior) e 70-90; vermelho em 45-60 e 115-140 (maior): 1 3 2 4 na tela.
    # Os maiores pedaços de cada cor ficam fora do intervalo da outra; ainda assim intercalam.
    imagem = np.full((68, 164, 3), 245, np.uint8)
    cv2.rectangle(imagem, (10, 15), (40, 55), (160, 40, 30), -1)
    cv2.rectangle(imagem, (70, 15), (90, 55), (160, 40, 30), -1)
    cv2.rectangle(imagem, (45, 15), (60, 55), (30, 30, 150), -1)
    cv2.rectangle(imagem, (115, 15), (140, 55), (30, 30, 150), -1)
    assert OcrLocal(ModeloPorPosicao).reconhecer(imagem) == []


def test_mesma_cor_ligada_por_cima_de_outra_camada_e_descartada():
    # Dois azuis ligados por um traço acima do vermelho viram um único pedaço: sem
    # intervalo entre centroides, a ordem dependeria só do centro e sairia trocada.
    imagem = np.full((68, 164, 3), 245, np.uint8)
    cv2.rectangle(imagem, (10, 15), (35, 55), (160, 40, 30), -1)
    cv2.rectangle(imagem, (130, 15), (155, 55), (160, 40, 30), -1)
    cv2.rectangle(imagem, (10, 8), (155, 16), (160, 40, 30), -1)
    cv2.rectangle(imagem, (75, 20), (90, 55), (30, 30, 150), -1)
    camadas = OcrLocal.camadas_por_cor(imagem)[0]  # k=2: azul e vermelho
    assert len(camadas) == 2
    # Testa a regra direto: o veto por divergência entre agrupamentos não pode ser a única proteção.
    assert _intercaladas([("99", .99), ("12", .99)], camadas)


def leitura(texto, modelo, variante, confianca=.97):
    return {"texto": texto, "confianca": confianca, "modelo": modelo, "variante": variante}


def test_modelos_divergentes_em_um_digito_nao_geram_candidato():
    # 1 vs 7 é a confusão mais comum: escolher pelo score enviaria um código errado.
    ocr = OcrLocal(Modelo)
    ocr.modelos[0][1].texto, ocr.modelos[1][1].texto = "1256", "7256"
    assert ocr.reconhecer(np.ones((20, 60, 3), np.uint8)) == []


def test_leitura_de_quatro_digitos_divergente_veta_mesmo_com_score_baixo():
    leituras = [leitura("7256", "padrao", "original"), leitura("7256", "beta", "original"),
                leitura("1256", "beta", "mascara", .30)]
    assert decidir(leituras, .90) == []


def test_mesma_familia_de_um_modelo_so_nao_basta():
    # original e cinza_2x do mesmo modelo erram juntos; não são fontes independentes.
    leituras = [leitura("7256", "padrao", "original"), leitura("7256", "padrao", "cinza_2x"),
                leitura("256", "beta", "original"), leitura("", "beta", "cinza_2x")]
    assert decidir(leituras, .90) == []


def test_um_modelo_em_duas_familias_sem_divergencia_e_aceito():
    leituras = [leitura("1256", "padrao", "original", .95), leitura("1256", "padrao", "mascara", .99),
                leitura("256", "beta", "original")]
    [c] = decidir(leituras, .90)
    assert (c.texto, c.modelo, c.variante, c.confianca) == ("1256", "padrao", "mascara", .99)


def test_dois_modelos_concordando_sao_aceitos_e_score_abaixo_do_limiar_nao_vota():
    leituras = [leitura("6723", "padrao", "cores", .954), leitura("6723", "beta", "cores", .989)]
    assert [c.texto for c in decidir(leituras, .90)] == ["6723"]
    leituras[0]["confianca"] = .80
    assert decidir(leituras, .90) == []


def test_mascara_remove_ruido_claro_e_mantem_digitos():
    mascara = dict(OcrLocal.variantes(captcha_sintetico()))["mascara"]
    assert set(np.unique(mascara)) == {0, 255}
    ys, xs = np.nonzero(mascara == 0)
    assert xs.min() >= 10 + 20 and xs.max() <= 10 + 140  # só os dois blocos coloridos


def test_modelos_reais_leem_o_captcha_da_especificacao():
    pytest.importorskip("ddddocr")
    from src.verificacao_visao import VisaoVerificacao
    raiz = Path(__file__).resolve().parents[1]
    frame = cv2.imread(str(raiz / "docs/especificacao/imagens/38_codigo_verificacao.png"))
    candidatos = OcrLocal().reconhecer(VisaoVerificacao().analisar(frame).codigo)
    assert candidatos and candidatos[0].texto == "6723"
    assert {c.variante for c in candidatos} == {"cores"}


@pytest.mark.parametrize("arquivo,codigo", [("codigo_real_3828.png", "3828"), ("codigo_real_5936.png", "5936")])
def test_maioria_aceita_codigo_real_com_uma_leitura_discordante(arquivo, codigo):
    # Recortes reais da run de 29/10 (logs/verificacao): só uma leitura discordava e a
    # unanimidade vetava um código certo.
    pytest.importorskip("ddddocr")
    imagem = cv2.imread(str(Path(__file__).resolve().parent / "imagens" / arquivo))
    candidatos = OcrLocal().reconhecer(imagem)
    assert [c.texto for c in candidatos] == [codigo]


@pytest.mark.parametrize("arquivo,codigo", [("codigo_real_7892.png", "7892"), ("codigo_real_6262.png", "6262")])
def test_codigos_reais_sem_maioria_nao_enviam_nada_errado(arquivo, codigo):
    pytest.importorskip("ddddocr")
    imagem = cv2.imread(str(Path(__file__).resolve().parent / "imagens" / arquivo))
    assert all(c.texto == codigo for c in OcrLocal().reconhecer(imagem))


def test_maioria_sem_os_dois_modelos_nao_basta():
    leituras = [leitura("5936", "beta", v) for v in ("original", "cinza_2x", "mascara")]
    leituras.append(leitura("5436", "padrao", "cores"))
    assert decidir(leituras) == []


@pytest.mark.parametrize("arquivo,codigo", [("codigo_real_2296.png", "2296"), ("codigo_real_6261.png", "6261"),
                                            ("codigo_real_6262.png", "6262")])
def test_palpite_acerta_captchas_reais_sem_certeza(arquivo, codigo):
    # Recortes reais (29–30/10): sem certeza pela regra de segurança, mas o palpite (mais
    # apoio somado entre as leituras de 4 dígitos) é o código certo.
    pytest.importorskip("ddddocr")
    from src.verificacao_ocr import palpite
    ocr = OcrLocal()
    assert ocr.reconhecer(cv2.imread(str(Path(__file__).resolve().parent / "imagens" / arquivo))) == []
    assert palpite(ocr.ultimas_leituras).texto == codigo


def test_palpite_soma_o_apoio_e_nao_repete_rejeitado():
    from src.verificacao_ocr import palpite
    leituras = [leitura("6261", "padrao", "original", .98), leitura("6261", "padrao", "cinza_2x", .99),
                leitura("6267", "beta", "original", .97), leitura("6267", "beta", "cinza_2x", .97),
                leitura("626", "beta", "mascara", 1.0)]
    assert palpite(leituras).texto == "6261"
    assert palpite(leituras, rejeitados={"6261"}).texto == "6267"
    assert palpite([leitura("225", "beta", "original", .99)]) is None  # sem 4 dígitos, sem palpite
