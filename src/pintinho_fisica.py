"""Força do tiro pelo ângulo, vento, distância e altura do alvo (modelo de trajetória do DDTank).

Portado do pacote ddtcurve 0.2.1 (PyPI, `ddtcurve/cal.py`, função `predict`), sem scipy: o
projétil sai com velocidade proporcional à força, sofre gravidade, vento e resistência do ar
proporcional à velocidade; as constantes são as calibradas pelo autor do pacote.
Unidades de distância: as das tabelas da moldura (o "1" da régua verde, ~100 px da tela).

Conferido em 30/10 (sem o jogo): com altura 0 fica a 1–3 de força das tabelas Âng. 20/30/50/65
da moldura; nas galinhas do estágio 2 da gravação, a −3…+2 das forças que mataram (uma a +6).
"""
import math

RESISTENCIA, VENTO, GRAVIDADE = 0.90289815, 6.33592869, -184.11666458
FORCA_MAX = 100.0


def _posicao(v0, f, t):
    """Deslocamento num eixo com velocidade inicial v0, aceleração f e resistência linear."""
    r = RESISTENCIA
    k = f - r * v0
    return (k * math.exp(-r * t) + f * r * t - k) / (r * r)


def _tempo_de_voo(vy, fy):
    """Primeiro t > 0 em que o projétil volta à linha do alvo (eixo y girado)."""
    t, passo = 0.05, 0.05
    if _posicao(vy, fy, t) <= 0:
        return None
    while _posicao(vy, fy, t + passo) > 0:
        t += passo
        if t > 60:
            return None
    baixo, alto = t, t + passo
    for _ in range(50):
        meio = (baixo + alto) / 2
        if _posicao(vy, fy, meio) > 0:
            baixo = meio
        else:
            alto = meio
    return (baixo + alto) / 2


def alcance(forca, angulo, vento, dx, dy):
    """Quanto o tiro anda ao longo da linha até o alvo (mesmas unidades de dx e dy)."""
    linha = math.atan2(dy, dx)
    tiro = math.radians(angulo)
    vx = math.cos(tiro - linha) * forca
    vy = math.cos(math.pi / 2 - tiro + linha) * forca
    fx = math.cos(linha) * VENTO * vento + math.sin(linha) * GRAVIDADE
    fy = -math.sin(linha) * VENTO * vento + math.cos(linha) * GRAVIDADE
    t = _tempo_de_voo(vy, fy)
    return None if t is None else _posicao(vx, fx, t)


def prever_forca(angulo, vento, dx, dy):
    """Força (0 a 100) para acertar um alvo a `dx` na horizontal e `dy` acima (negativo: abaixo).

    `vento`: positivo a favor do tiro, negativo contra (como no relógio do vento, em unidades).
    None se nem com força 100 o tiro chega (ou o ângulo não sobe acima da linha do alvo).
    """
    if dx <= 0:
        return None
    if angulo > 90:
        angulo = 180 - angulo
    distancia = math.hypot(dx, dy)
    baixo, alto = 1.0, 150.0
    a_alto = alcance(alto, angulo, vento, dx, dy)
    if a_alto is None or a_alto < distancia:
        return None
    for _ in range(60):
        meio = (baixo + alto) / 2
        a = alcance(meio, angulo, vento, dx, dy)
        if a is not None and a >= distancia:
            alto = meio
        else:
            baixo = meio
    forca = (baixo + alto) / 2
    return forca if forca <= FORCA_MAX else None
