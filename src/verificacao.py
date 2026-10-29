"""Código de verificação (anti-bot) do jogo: o bot NÃO resolve. Ele para, avisa e espera.

Quando o quadro "Código de verificação" aparece, o bot traz a janela para frente, apita
e mostra um aviso até a pessoa digitar o código. Quando o quadro some, o bot continua.
"""
import time
from datetime import datetime

from . import vision
from .telas import IMG

CODIGO = IMG / "codigo_verificacao.png"
LIMIAR_CODIGO = 0.85
ESPERA_MAX = 180  # segundos esperando a pessoa resolver antes de desistir de esperar


def tem_codigo(frame):
    return vision.achar(CODIGO, frame, LIMIAR_CODIGO) is not None


def apitar():
    try:
        import winsound
        winsound.Beep(1200, 250)
        winsound.Beep(900, 250)
    except Exception:
        print("\a", end="", flush=True)


def esperar_pessoa(driver, nome="bot"):
    """Para tudo até a pessoa resolver o código na janela deste driver.

    Não clica em nada dentro do quadro: só traz a janela para frente e avisa.
    """
    from .drivers.tela import trazer_para_frente
    if getattr(driver, "hwnd", None):
        trazer_para_frente(driver.hwnd)
    aviso = (f"[{datetime.now():%H:%M:%S}] [{nome}] *** CÓDIGO DE VERIFICAÇÃO NA TELA: "
             f"digite o código no jogo. O bot está parado esperando. ***")
    print(aviso, flush=True)
    limite = time.time() + ESPERA_MAX
    while time.time() < limite:
        apitar()
        time.sleep(1.5)
        if not tem_codigo(driver.capturar()):
            print(f"[{datetime.now():%H:%M:%S}] [{nome}] código resolvido: continuando", flush=True)
            time.sleep(1)
            return True
    print(f"[{datetime.now():%H:%M:%S}] [{nome}] o código continua na tela depois de "
          f"{ESPERA_MAX} s; sigo esperando na próxima volta", flush=True)
    return False
