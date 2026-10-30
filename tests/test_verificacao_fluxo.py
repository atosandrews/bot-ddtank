from types import SimpleNamespace

import numpy as np
import pytest

from src import config
from src.verificacao import resolver
from src.verificacao_ocr import Candidato, ErroInfraestrutura


class Relogio:
    agora = 0.0

    def dormir(self, segundos):
        self.agora += segundos

    def __call__(self):
        return self.agora


class Driver:
    def __init__(self, alvo="0011", ocupados="", reset_muda=False):
        self.alvo = alvo
        self.entrada = ocupados
        self.fechada = self.fim = False
        self.identidade = "A"
        self.confirmados = []
        self.cliques = []
        self.capturas = 0
        self.foco_falha = 0
        self.ao_clicar = None
        self.reset_muda = reset_muda

    def janela_valida(self):
        return not self.fechada

    def preparar_verificacao(self):
        if self.foco_falha:
            self.foco_falha -= 1
            return False
        return True

    def capturar_verificacao(self):
        self.capturas += 1
        assert self.capturas < 12000, "Fluxo não terminou no cenário simulado"
        return dict(id=self.identidade, entrada=self.entrada, fim=self.fim), {"left": 0}

    def clicar_verificado(self, x, y, *, regiao_esperada):
        assert regiao_esperada == {"left": 0}
        self.cliques.append((x, y))
        if y == 10:
            self.entrada += str(x)
        elif x == 100:
            self.confirmados.append(self.entrada)
            self.fim = self.entrada == self.alvo
        elif x == 200:
            self.entrada = ""
            if self.reset_muda:
                self.identidade = "B"
        if self.ao_clicar:
            self.ao_clicar(self, x, y)
        return True


class Visao:
    def analisar(self, frame):
        if frame["fim"]:
            return None
        return SimpleNamespace(identidade=frame["id"], origem=(0, 0),
            codigo=np.array([ord(frame["id"])]), tempo=np.array([1]),
            teclas={str(i): (i, 10) for i in range(10)},
            confirmar=(100, 100), resetar=(200, 100), ocupados=len(frame["entrada"]))


class Ocr:
    def __init__(self, textos=("0011",)):
        self.textos = textos
        self.leituras = 0

    def reconhecer(self, imagem, limiar):
        self.leituras += 1
        return [Candidato(t, .99, "simulado", "original") for t in self.textos]

    def ler_tempo(self, imagem):
        return 30


@pytest.fixture(autouse=True)
def parametros(monkeypatch):
    monkeypatch.setattr(config, "VERIFICACAO_INTERVALO", .01)
    monkeypatch.setattr(config, "VERIFICACAO_ESPERA_CLIQUE", .01)
    monkeypatch.setattr(config, "VERIFICACAO_ESPERA_RESPOSTA", .02)
    monkeypatch.setattr(config, "VERIFICACAO_DURACAO_DESAFIO", 10000)


def executar(driver, ocr=None, **kwargs):
    relogio = Relogio()
    return resolver(driver, ocr=ocr or Ocr(), visao=Visao(), dormir=relogio.dormir,
        relogio=relogio, log=lambda *a, **k: None, tela_conhecida=lambda f: f["fim"], **kwargs)


def test_preenche_zero_repetido_e_confirma():
    d = Driver()
    r = executar(d)
    assert (r.estado, r.tentativas) == ("RESOLVIDO", 1)
    assert d.confirmados == ["0011"]
    assert d.cliques == [(0, 10), (0, 10), (1, 10), (1, 10), (100, 100)]


def test_rejeicao_reseta_e_tenta_outro_candidato_sem_repetir_o_primeiro():
    d = Driver(alvo="0022")
    r = executar(d, Ocr(("0011", "0022")))
    assert r.tentativas == 2
    assert d.confirmados == ["0011", "0022"]
    assert (200, 100) in d.cliques


def test_sem_limite_fixo_de_tentativas():
    d = Driver(alvo="0100")
    r = executar(d, Ocr(tuple(f"{i:04d}" for i in range(101))))
    assert r.tentativas == 101
    assert len(set(d.confirmados)) == 101


def test_reset_com_novo_desafio_descarta_a_leitura_anterior():
    d = Driver(ocupados="12", reset_muda=True)
    r = executar(d)
    assert r.estado == "RESOLVIDO"
    assert d.identidade == "B" and d.confirmados == ["0011"]


def test_mudanca_durante_preenchimento_abandona_sequencia_antiga():
    d = Driver(alvo="0022")
    def mudar(driver, x, y):
        if driver.identidade == "A" and y == 10:
            driver.identidade, driver.entrada = "B", ""
    d.ao_clicar = mudar
    class OcrMutavel(Ocr):
        def reconhecer(self, imagem, limiar):
            return [Candidato("0011" if imagem[0] == ord("A") else "0022", .99, "m", "v")]
    executar(d, OcrMutavel())
    assert d.confirmados == ["0022"]


def test_perda_temporaria_de_foco_nao_envia_cliques_antecipados():
    d = Driver()
    d.foco_falha = 3
    executar(d)
    assert d.confirmados == ["0011"]


def test_leitura_incompleta_recaptura_sem_clicar():
    d = Driver()
    class OcrIncerto(Ocr):
        def reconhecer(self, imagem, limiar):
            self.leituras += 1
            if self.leituras < 4:
                assert not d.cliques
                return []
            return [Candidato("0011", .99, "m", "v")]
    ocr = OcrIncerto()
    executar(d, ocr)
    assert ocr.leituras == 4


def test_expiracao_nao_e_contada_como_sucesso():
    d = Driver()
    class OcrExpirado(Ocr):
        def ler_tempo(self, imagem):
            d.fim = True
            return 0
    r = executar(d, OcrExpirado())
    assert r.estado == "EXPIRADO" and not d.cliques


def test_janela_fechada_e_erro_estrutural():
    d = Driver()
    d.fechada = True
    with pytest.raises(ErroInfraestrutura, match="fechada"):
        executar(d)


def test_ctrl_c_nao_e_capturado():
    d = Driver()
    def interromper(segundos):
        raise KeyboardInterrupt
    with pytest.raises(KeyboardInterrupt):
        resolver(d, ocr=Ocr(()), visao=Visao(), dormir=interromper,
                 log=lambda *a, **k: None)


def test_candidatos_esgotados_nao_reenviam_codigo_rejeitado():
    d = Driver(alvo="0022")
    class OcrNovoDesafio(Ocr):
        def reconhecer(self, imagem, limiar):
            self.leituras += 1
            if self.leituras == 4:
                assert d.confirmados == ["0011"]
                d.identidade = "B"
            texto = "0011" if imagem[0] == ord("A") else "0022"
            return [Candidato(texto, .99, "m", "v")]
    executar(d, OcrNovoDesafio())
    assert d.confirmados == ["0011", "0022"]


def test_fim_do_prazo_durante_inferencia_impede_preenchimento():
    d = Driver()
    relogio = Relogio()
    class OcrLento(Ocr):
        def reconhecer(self, imagem, limiar):
            relogio.dormir(31)
            d.fim = True
            return super().reconhecer(imagem, limiar)
    resultado = resolver(d, ocr=OcrLento(), visao=Visao(), dormir=relogio.dormir,
        relogio=relogio, log=lambda *a, **k: None, tela_conhecida=lambda f: f["fim"])
    assert resultado.estado == "EXPIRADO"
    assert not d.cliques


def test_desaparecimento_nao_e_sucesso_sem_duas_capturas_conhecidas():
    d = Driver()
    d.fim = True
    conhecidos = iter((False, True, False, True, True))
    relogio = Relogio()
    resultado = resolver(d, ocr=Ocr(), visao=Visao(), dormir=relogio.dormir,
        relogio=relogio, log=lambda *a, **k: None, tela_conhecida=lambda f: next(conhecidos))
    assert d.capturas == 5
    assert resultado.estado == "RETOMADO" and not d.cliques


def test_entrada_parcial_e_limpada_antes_da_primeira_tentativa():
    d = Driver(ocupados="23")
    executar(d)
    assert d.cliques[0] == (200, 100)
    assert d.confirmados == ["0011"]


def test_erro_de_inferencia_nao_vira_loop_de_tentativas():
    d = Driver()
    class OcrQuebrado(Ocr):
        def reconhecer(self, imagem, limiar):
            raise ErroInfraestrutura("modelo indisponível")
    with pytest.raises(ErroInfraestrutura, match="modelo indisponível"):
        executar(d, OcrQuebrado())
    assert not d.cliques


def test_rejeicao_de_uma_conta_nao_contamina_a_outra():
    a, b = Driver(alvo="0022"), Driver(alvo="0011")
    ocr = Ocr(("0011", "0022"))
    executar(a, ocr)
    executar(b, ocr)
    assert a.confirmados == ["0011", "0022"]
    assert b.confirmados == ["0011"]


def test_controlador_publico_serializa_contas(monkeypatch):
    from src import verificacao
    def simular(*args):
        assert verificacao._EXCLUSAO.locked()
        return verificacao.Resultado("RESOLVIDO", 1)
    monkeypatch.setattr(verificacao, "resolver", simular)
    assert verificacao.tratar(Driver()).estado == "RESOLVIDO"


def test_perda_de_foco_apos_primeiro_numero_limpa_e_recomeca():
    d = Driver()
    interrompido = False
    def perder_foco(driver, x, y):
        nonlocal interrompido
        if y == 10 and not interrompido:
            interrompido = True
            driver.foco_falha = 1
    d.ao_clicar = perder_foco
    executar(d)
    assert d.confirmados == ["0011"]
    assert (200, 100) in d.cliques


def test_diagnostico_salva_desafio_sem_candidato_uma_vez_e_o_envio(monkeypatch, tmp_path):
    import json
    from src import verificacao
    monkeypatch.setattr(config, "VERIFICACAO_SALVAR_DIAGNOSTICO", True)
    monkeypatch.setattr(verificacao, "PASTA_DIAGNOSTICO", tmp_path)

    class VisaoComImagem(Visao):
        def analisar(self, frame):
            quadro = super().analisar(frame)
            if quadro is not None:
                quadro.codigo = np.zeros((4, 4, 3), np.uint8)
            return quadro

    class OcrHesitante(Ocr):
        def reconhecer(self, imagem, limiar):
            self.leituras += 1
            self.ultimas_leituras = [{"texto": "0011", "confianca": np.float32(.5),
                                      "modelo": "m", "variante": "v"}]
            return [] if self.leituras <= 3 else super().reconhecer(imagem, limiar)

    d, relogio = Driver(), Relogio()
    r = resolver(d, ocr=OcrHesitante(), visao=VisaoComImagem(), dormir=relogio.dormir,
                 relogio=relogio, log=lambda *a, **k: None, tela_conhecida=lambda f: f["fim"])
    assert r.estado == "RESOLVIDO"
    sem = sorted(tmp_path.glob("*_sem_candidato.json"))
    assert len(sem) == 1  # três leituras vazias da mesma imagem, um arquivo
    dados = json.loads(sem[0].read_text(encoding="utf-8"))
    assert dados["candidato"] is None and dados["leituras"][0]["texto"] == "0011"
    enviados = [p for p in tmp_path.glob("*.json") if "sem_candidato" not in p.name]
    assert json.loads(enviados[0].read_text(encoding="utf-8"))["candidato"] == "0011"
    assert len(list(tmp_path.glob("*.png"))) == 2


def test_sem_certeza_envia_o_melhor_palpite(monkeypatch):
    monkeypatch.setattr(config, "VERIFICACAO_ENVIAR_PALPITE", True)

    class OcrSemCerteza(Ocr):
        def reconhecer(self, imagem, limiar):
            self.ultimas_leituras = [{"texto": "0011", "confianca": .97, "modelo": "beta", "variante": "original"},
                                     {"texto": "011", "confianca": .99, "modelo": "padrao", "variante": "original"}]
            return []
    d = Driver()
    r = executar(d, OcrSemCerteza())
    assert r.estado == "RESOLVIDO" and d.confirmados == ["0011"]


def test_palpite_desligado_espera(monkeypatch):
    monkeypatch.setattr(config, "VERIFICACAO_ENVIAR_PALPITE", False)

    class OcrSemCerteza(Ocr):
        def reconhecer(self, imagem, limiar):
            self.leituras += 1
            if self.leituras > 5:
                d.fim = True
            self.ultimas_leituras = [{"texto": "0011", "confianca": .97, "modelo": "beta", "variante": "original"}]
            return []
    d = Driver()
    executar(d, OcrSemCerteza())
    assert d.confirmados == []
