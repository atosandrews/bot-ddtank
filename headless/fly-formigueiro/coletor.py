"""Roda DENTRO da máquina do Fly (enviado pelo painel_fly.py a cada atualização).
Imprime uma linha JSON: {"estado", "login": [linhas JSON do login], "bot": {id: status}}."""
import json
import subprocess

CONSULTA = r'''__CONSULTA__'''
CONTROLE = ["timeout", "12", "/app/ddrelay", "control", "-socket", "/tmp/ddctl/ddrelay.sock"]
saida = {"estado": "running", "login": [], "bot": {}}
try:
    saida["login"] = [l for l in open("/tmp/login.log", encoding="utf-8", errors="replace").read().splitlines()
                      if l.startswith("{") and '"cookie"' not in l]
except OSError:
    saida["estado"] = "iniciando"
try:
    sessoes = json.loads(subprocess.run(CONTROLE + ["-sessions"], capture_output=True, text=True).stdout)["value"]
except (json.JSONDecodeError, KeyError, TypeError):
    sessoes = []
for sessao in sessoes:
    r = subprocess.run(CONTROLE + ["-session", str(sessao["session"]), "-eval", CONSULTA], capture_output=True, text=True)
    try:
        valor = json.loads(r.stdout.splitlines()[0])["value"]
    except (json.JSONDecodeError, KeyError, IndexError, TypeError):
        continue
    if isinstance(valor, dict) and valor.get("id"):
        saida["bot"][str(valor["id"])] = valor
print(json.dumps(saida, ensure_ascii=False))
