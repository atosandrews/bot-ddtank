"""Mostra no PC os painéis que rodam DENTRO das máquinas do Fly (10/11/2026).

A contagem do farm e o painel.html são feitos lá (em /data/painel, no volume), então continuam com o PC
desligado. Este script só baixa o painel.html pronto de cada app a cada 60 s e grava a cópia local que o
navegador abre. Sem janela: rode com pythonw. Não conta nada e não grava farm.json.
"""
import subprocess
import time
from datetime import datetime
from pathlib import Path

DDBOT = Path(__file__).parent
FLY = str(Path.home() / ".fly" / "bin" / "flyctl.exe")
ATUALIZAR = 60
PAINEIS = {
    "ddbot-formigueiro": DDBOT / "fly-formigueiro" / "painel.html",
    "ddbot-pintinho": DDBOT / "pintinho" / "painel.html",
    "ddbot-olimpiadas": DDBOT / "olimpiadas" / "painel.html",  # 13/11
}


def baixar(app, destino):
    try:
        saida = subprocess.run([FLY, "ssh", "console", "-a", app, "-C", "cat /data/painel/painel.html"],
                               capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=90,
                               creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0))
    except subprocess.TimeoutExpired:
        return "sem resposta"
    inicio = saida.stdout.find("<!doctype html>")
    if inicio < 0:
        return "painel ainda não gerado na máquina"
    temporario = destino.with_suffix(".novo")
    temporario.write_text(saida.stdout[inicio:], encoding="utf-8")
    temporario.replace(destino)
    return "ok"


while True:
    for app, destino in PAINEIS.items():
        try:
            resultado = baixar(app, destino)
        except Exception as erro:  # noqa: BLE001 - uma falha (arquivo preso, rede) não pode parar o leitor
            resultado = f"falha: {erro}"
        print(f"[{datetime.now():%H:%M:%S}] {app}: {resultado}", flush=True)
    time.sleep(ATUALIZAR)
