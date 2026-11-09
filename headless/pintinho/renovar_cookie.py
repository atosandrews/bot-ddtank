"""Renova o cookie .Auth da conta: o site devolve um .Auth novo, válido por 14 dias, a cada página
visitada. Guarda o cookie renovado na sessão em uso (/tmp/cfg/sessao.json) e numa cópia que
sobrevive a reinícios (/ws/run/sessao_atual.json). Nunca imprime o valor do cookie.

Uso: python3 -I renovar_cookie.py
"""
import json
import os
import sys
import urllib.request
from http.cookies import SimpleCookie

EM_USO = sys.argv[1] if len(sys.argv) > 1 else "/tmp/cfg/sessao.json"
PERSISTENTE = sys.argv[2] if len(sys.argv) > 2 else "/cache/sessao_pintinho.json"
PAGINA = "https://ddclassico.com/classic/playgame"
AGENTE = ("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) DDClassico/1.0.5 "
          "Chrome/87.0.4280.141 Electron/11.5.0 Safari/537.36 DDTankClassico/0.1.0.0 DDClassico/0.1.0.0")


def gravar(caminho, cookie):
    temporario = caminho + ".novo"
    with open(temporario, "w", encoding="utf-8") as arquivo:
        json.dump({"cookie": cookie}, arquivo)
    os.chmod(temporario, 0o600)
    os.replace(temporario, caminho)


def main():
    with open(EM_USO, encoding="utf-8") as arquivo:
        atual = json.load(arquivo)["cookie"]
    pedido = urllib.request.Request(PAGINA, headers={"User-Agent": AGENTE, "Cookie": ".Auth=" + atual})
    with urllib.request.urlopen(pedido, timeout=30) as resposta:
        if resposta.status != 200 or resposta.geturl() != PAGINA:
            print(f"cookie: página respondeu {resposta.status} em {resposta.geturl()}; sessão pode ter expirado")
            return 1
        novo, validade = None, "?"
        for cabecalho in resposta.headers.get_all("Set-Cookie") or []:
            biscoito = SimpleCookie()
            biscoito.load(cabecalho)
            if ".Auth" in biscoito:
                novo, validade = biscoito[".Auth"].value, biscoito[".Auth"]["expires"] or "?"
    if not novo or len(novo) < 16:
        print("cookie: o site não devolveu um .Auth novo; mantido o atual")
        return 1
    gravar(EM_USO, novo)
    gravar(PERSISTENTE, novo)
    print(f"cookie renovado; válido até {validade}")
    return 0


try:
    sys.exit(main())
except Exception as erro:  # noqa: BLE001 - renovar nunca pode derrubar o grupo
    print(f"cookie: renovação falhou ({type(erro).__name__}); mantido o atual")
    sys.exit(1)
