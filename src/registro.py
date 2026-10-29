"""Cópia do log do terminal em arquivo: o terminal some ao fechar e a run não pode ser revista."""
from datetime import datetime
from pathlib import Path

LOGS = Path(__file__).resolve().parent.parent / "logs"


def gravar_log(linha):
    """Acrescenta a linha em logs/execucao_AAAAMMDD.txt; falha de disco não para o bot."""
    try:
        LOGS.mkdir(exist_ok=True)
        with open(LOGS / f"execucao_{datetime.now():%Y%m%d}.txt", "a", encoding="utf-8") as f:
            f.write(linha + "\n")
    except OSError:
        pass


def imprimir(*partes, **kwargs):
    """Substituto de print que também grava no arquivo de execução."""
    print(*partes, **kwargs)
    gravar_log(f"[{datetime.now():%H:%M:%S}] " + " ".join(map(str, partes)))
