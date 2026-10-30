"""Roda de números girando (29/10): clique adiantado pelo giro medido e mouse no centro.

Captcha falso: a roda gira a uma velocidade conhecida e cada print avança o relógio.
Os números ficam a 36° um do outro, decrescentes no sentido horário, como no jogo.
"""
from types import SimpleNamespace

import numpy as np
import pytest

from src import config
from src.verificacao import resolver
from src.verificacao_ocr import Candidato
from src.verificacao_visao import VisaoVerificacao

CENTRO, RAIO = (500, 300), 100
REPOUSO = (300, 150)  # barra do título, fora do teclado


class Relogio:
    agora = 0.0

    def dormir(self, s):
        self.agora += s

    def __call__(self):
        return self.agora


def posicao(digito, angulo):
    """Posição do número na roda girada `angulo` graus (0 no 0 à direita)."""
    a = np.radians(angulo - 36 * int(digito))  # decrescente no horário
    return CENTRO[0] + RAIO * np.cos(a), CENTRO[1] + RAIO * np.sin(a)


class Driver:
    def __init__(self, relogio, velocidade, alvo="0123"):
        self.relogio, self.velocidade, self.alvo = relogio, velocidade, alvo
        self.entrada, self.fim = "", False
        self.cliques, self.estacionado = [], []

    def angulo(self):
        return self.velocidade * self.relogio()

    def janela_valida(self):
        return True

    def preparar_verificacao(self):
        return True

    def estacionar_mouse(self, x, y):
        self.estacionado.append((x, y))

    def capturar_verificacao(self):
        self.relogio.agora += 0.05  # cada print leva um tempo
        self.prints = getattr(self, "prints", 0) + 1
        if self.prints > 3000:  # o desafio "acaba" para o teste não ficar preso
            self.fim = True
        return dict(angulo=self.angulo(), entrada=self.entrada, fim=self.fim), {"left": 0}

    def clicar_verificado(self, x, y, *, regiao_esperada, estacionar=None):
        # Onde cada número está no instante do clique (depois da latência configurada).
        angulo = self.velocidade * (self.relogio() + config.VERIFICACAO_LATENCIA_CLIQUE)
        self.cliques.append((x, y, estacionar))
        if (x, y) == (100, 100):
            self.fim = self.entrada == self.alvo
            return True
        mais_perto = min("0123456789", key=lambda d: np.hypot(*np.subtract(posicao(d, angulo), (x, y))))
        if np.hypot(*np.subtract(posicao(mais_perto, angulo), (x, y))) <= 6:
            self.entrada += mais_perto
        return True


class Visao:
    girar = staticmethod(VisaoVerificacao.girar)

    def analisar(self, frame):
        if frame["fim"]:
            return None
        a = frame["angulo"]
        return SimpleNamespace(
            identidade="A", origem=(0, 0), codigo=np.zeros((4, 4, 3)), tempo=np.zeros(1),
            teclas={d: tuple(int(round(v)) for v in posicao(d, a)) for d in "0123456789"},
            confirmar=(100, 100), resetar=(200, 100), ocupados=len(frame["entrada"]),
            centro_roda=CENTRO, fase=a % 36, repouso=REPOUSO)

    def fase(self, frame, origem):
        return frame["angulo"] % 36


class Ocr:
    def reconhecer(self, imagem, limiar):
        return [Candidato("0123", .99, "simulado", "cores")]

    def ler_tempo(self, imagem):
        return 30


@pytest.fixture(autouse=True)
def parametros(monkeypatch):
    monkeypatch.setattr(config, "VERIFICACAO_INTERVALO", .01)
    monkeypatch.setattr(config, "VERIFICACAO_ESPERA_CLIQUE", .01)
    monkeypatch.setattr(config, "VERIFICACAO_ESPERA_RESPOSTA", .02)
    monkeypatch.setattr(config, "VERIFICACAO_DURACAO_DESAFIO", 10000)


def executar(driver, relogio, logs):
    return resolver(driver, ocr=Ocr(), visao=Visao(), dormir=relogio.dormir, relogio=relogio,
                    log=lambda *a, **k: logs.append(a[0]), tela_conhecida=lambda f: f["fim"])


def test_roda_girando_clica_onde_o_numero_vai_estar():
    relogio, logs = Relogio(), []
    d = Driver(relogio, velocidade=40)  # 40°/s: sem adiantar, erraria o botão
    r = executar(d, relogio, logs)
    assert r.estado == "RESOLVIDO" and d.entrada == "0123"
    assert any("roda +40°/s" in l for l in logs)


def test_sem_adiantar_o_giro_o_clique_erraria():
    # Confere que o teste acima depende mesmo da previsão: o número sai do lugar.
    deslocamento = np.hypot(*np.subtract(posicao("0", 40 * 0.0), posicao("0", 40 * 0.17)))
    assert deslocamento > 6


def test_roda_rapida_demais_nao_e_clicada(monkeypatch):
    monkeypatch.setattr(config, "VERIFICACAO_GIRO_MAX", 30)
    relogio, logs = Relogio(), []
    d = Driver(relogio, velocidade=40)
    d.capturar_original = d.capturar_verificacao
    chamadas = {"n": 0}

    def limitar():
        chamadas["n"] += 1
        if chamadas["n"] > 40:  # depois de várias recusas, encerra o desafio no teste
            d.fim = True
        return d.capturar_original()
    d.capturar_verificacao = limitar
    executar(d, relogio, logs)
    assert d.entrada == "" and any("rápida demais" in l for l in logs)


def test_mouse_espera_fora_do_teclado():
    # Run de 29/10: com o mouse no centro da roda os botões sumiram (0 botões soltos).
    relogio, logs = Relogio(), []
    d = Driver(relogio, velocidade=0)
    executar(d, relogio, logs)
    assert d.estacionado and set(d.estacionado) == {REPOUSO}
    assert all(c[2] == REPOUSO for c in d.cliques)  # depois de cada clique volta para fora
