import sys

from . import config
from .bot import Bot
from .drivers import DriverTela
from .drivers.tela import listar_janelas


def escolher_titulo():
    if len(sys.argv) > 1:
        return sys.argv[1]
    if config.TITULO_JANELA:
        return config.TITULO_JANELA
    print("Janelas abertas:")
    for hwnd, titulo in listar_janelas():
        print(f"  {hwnd} | {titulo}")
    print("Dica: salve o título em src/config.py (TITULO_JANELA) para não perguntar de novo.")
    return input("Digite parte do título da janela do jogo: ").strip()


if __name__ == "__main__":
    titulo = escolher_titulo()
    if not titulo:
        print('Uso: python -m src.main "TITULO DA JANELA"')
        sys.exit(1)
    try:
        driver = DriverTela.da_janela(titulo)
    except RuntimeError as e:
        print(e)
        sys.exit(1)
    sys.exit(0 if Bot(driver, "bot1").rodar() else 1)
