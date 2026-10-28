import sys

from .bot import Bot
from .drivers import DriverTela

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print('Uso: python -m src.main "TITULO DA JANELA"')
        sys.exit(1)
    driver = DriverTela.da_janela(sys.argv[1])
    Bot(driver, "bot1").rodar()
