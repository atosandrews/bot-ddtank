"""Gera painel.html com o estado de cada personagem a partir dos logs do contêiner ddrelay-headless.

Uso: py -3.12 painel.py  (depois renderize painel.html como imagem)
Lutas e recompensas só aparecem quando o bot jogar instâncias; o modo login apenas mantém os personagens online.
"""
import html
import json
import re
import subprocess
import sys
import time
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path

PASTA = Path(__file__).parent
CONTAINER = "ddbot-grupo"
ATUALIZAR = 15  # segundos entre atualizações no modo --ao-vivo
CONTROLE = ["docker", "exec", CONTAINER, "/tmp/ddrelay", "control", "-socket", "/tmp/ddctl/ddrelay.sock"]
# Contadores vêm de grupo_status().total, que soma os Formigueiros recriados pela recuperação.
CONSULTA = ("local s = formigueiro_status and formigueiro_status() or {}; "
            "local g = grupo_status and grupo_status() or {}; local t = g.total or {}; "
            "return {id = player.state().id, phase = s.phase, reason = s.reason, "
            "runs = t.runs, wins = t.wins, losses = t.losses, deliveries = t.deliveries, "
            "grupo = {mochila = g.mochila, recuperacoes = g.recuperacoes, captchas = g.captchas and {aceitos = g.captchas.aceitos, recusados = g.captchas.recusados}, carteira = g.carteira}}")
ITENS = Path(r"C:\Users\ATOS I\Downloads\bot\bot\lua\data\items")
OURO_GANHO = "Ouro ganho na carteira"


def nomes_itens():
    """Mapa template -> nome, lido dos arquivos de dados do bot."""
    nomes = {}
    for arquivo in ITENS.rglob("*.lua"):
        texto = arquivo.read_text(encoding="utf-8", errors="replace")[:400]
        ident = re.search(r"\bid = (-?\d+)", texto)  # moedas de ouro usam -100
        nome = re.search(r'\bname = "([^"]*)"', texto)
        if ident and nome:
            nomes[ident.group(1)] = nome.group(1)
    return nomes


def formigueiro():
    """Fase e contadores do Formigueiro de cada personagem, pela sessão do relay."""
    por_id = {}
    saida = subprocess.run(CONTROLE + ["-sessions"], capture_output=True, text=True)
    try:
        sessoes = json.loads(saida.stdout)["value"]
    except (json.JSONDecodeError, KeyError):
        return por_id
    for sessao in sessoes:
        resposta = subprocess.run(CONTROLE + ["-session", str(sessao["session"]), "-eval", CONSULTA], capture_output=True, text=True)
        try:
            valor = json.loads(resposta.stdout.splitlines()[0])["value"]
        except (json.JSONDecodeError, KeyError, IndexError):
            continue
        if valor.get("id"):
            por_id[valor["id"]] = valor
    return por_id


def drops(deliveries, nomes):
    """Soma os itens recebidos em todas as partidas concluídas: [(nome, quantidade)]."""
    soma = Counter()
    if isinstance(deliveries, dict):
        for entrega in deliveries.values():
            recebidos = entrega.get("received") if isinstance(entrega, dict) else None
            if isinstance(recebidos, dict):
                for template, quantidade in recebidos.items():
                    if isinstance(quantidade, (int, float)):
                        soma[nomes.get(str(template), f"item {template}")] += int(quantidade)
    return sorted(soma.items())


def personagens():
    lista, atual = [], {}
    for linha in (PASTA / "headless.yaml").read_text(encoding="utf-8").splitlines():
        linha = linha.strip()
        if linha.startswith("- name:"):
            atual = {"name": linha.split(":", 1)[1].strip()}
            lista.append(atual)
        elif atual and ":" in linha and not linha.startswith("#"):
            chave, valor = (p.strip() for p in linha.split(":", 1))
            atual[chave] = valor
    return lista


def eventos():
    saida = subprocess.run(["docker", "logs", CONTAINER], capture_output=True, text=True, encoding="utf-8", errors="replace")
    estado = subprocess.run(["docker", "inspect", "-f", "{{.State.Status}}", CONTAINER], capture_output=True, text=True).stdout.strip()
    linhas = []
    for linha in (saida.stdout + saida.stderr).splitlines():
        if linha.startswith("{"):
            try:
                linhas.append(json.loads(linha))
            except json.JSONDecodeError:
                pass
    return estado or "parado", linhas


def hora(texto):
    return datetime.fromisoformat(texto[:26].rstrip("Z")).replace(tzinfo=timezone.utc).astimezone()


def main():
    estado, linhas = eventos()
    bot = formigueiro() if estado == "running" else {}
    nomes = nomes_itens()
    agora = datetime.now().astimezone()
    painel = []
    total = Counter()
    itens_total = Counter()
    for p in personagens():
        cid = int(p["id"])
        meus = [e for e in linhas if e.get("character_id") == cid]
        http = next((e for e in meus if e["msg"] == "headless HTTP login confirmed"), None)
        jogo = next((e for e in meus if e["msg"] == "headless game login confirmed"), None)
        erros = [e for e in meus if e.get("level") == "ERROR"]
        if erros:
            status = "erro: " + erros[-1].get("error", "?")
        elif jogo and estado == "running":
            status = "online"
        elif jogo:
            status = "desconectado"
        else:
            status = "entrando..."
        linhas_p = [
            f"Status: {status}",
            f"ID no jogo: {cid}",
            f"Número do site: {p['website_id']}",
            f"Servidor: {p['upstream']}",
            f"Login HTTP: {hora(http['time']):%H:%M:%S}" if http else "Login HTTP: -",
            f"Login no jogo: {hora(jogo['time']):%H:%M:%S}" if jogo else "Login no jogo: -",
        ]
        if jogo and estado == "running":
            minutos = int((agora - hora(jogo["time"])).total_seconds() // 60)
            linhas_p.append(f"Tempo online: {minutos // 60}h{minutos % 60:02d}min")
            total["online"] += 1
        info = bot.get(cid, {})
        runs = int(info.get("runs") or 0)
        grupo = info.get("grupo") or {}
        mochila = grupo.get("mochila") or {}
        if mochila:
            linhas_p.append(f"Mochila: {mochila.get('itens', 0)} itens, {mochila.get('equipamentos', 0)} equipamentos")
        if mochila.get("cheia"):
            linhas_p.append(f"!! {mochila['cheia']}")
            total["alertas"] += 1
        captchas = grupo.get("captchas") or {}
        aceitos, recusados = int(captchas.get("aceitos") or 0), int(captchas.get("recusados") or 0)
        total["captcha_ok"] += aceitos
        total["captcha_erro"] += recusados
        if aceitos or recusados:
            linhas_p.append(f"Captchas: {aceitos} aceitos, {recusados} recusados")
        recuperacoes = grupo.get("recuperacoes") or []
        if recuperacoes:
            linhas_p.append(f"Recuperações automáticas: {len(recuperacoes)} (última: {recuperacoes[0]})")
        linhas_p += ["", f"Fase: {info.get('phase', '-')}"]
        if info.get("reason"):
            linhas_p.append(f"Motivo: {info['reason']}")
        itens = drops(info.get("deliveries"), nomes)
        carteira = grupo.get("carteira") or {}
        if isinstance(carteira.get("gold"), (int, float)):
            ganho = int(carteira["gold"] - carteira.get("gold_inicio", carteira["gold"]))
            linhas_p.append(f"Ouro na carteira: {int(carteira['gold']):,}".replace(",", "."))
            # Guardado como item para somar entre reinícios do grupo (cada sessão parte do zero).
            itens = list(itens) + [(OURO_GANHO, max(ganho, 0))]
        if info:
            farm = acumular(p["name"], {"runs": runs, "wins": int(info.get("wins") or 0),
                                        "losses": int(info.get("losses") or 0), "itens": dict(itens)})
        else:
            farm = farm_de(p["name"])
        total["fights"] = max(total["fights"], farm["runs"])
        linhas_p += ["", f"Fights desde {inicio_farm():%d/%m %H:%M}: {farm['runs']}  "
                         f"(vitórias {farm['wins']}, derrotas {farm['losses']})"]
        for nome, quantidade in sorted(farm["itens"].items()):
            linhas_p.append(f"{nome} × {quantidade}")
            itens_total[nome] += quantidade
        if not farm["itens"]:
            linhas_p.append("Recompensas: nenhuma ainda")
        painel.append((p["name"], linhas_p))

    n = len(painel)
    horas = (agora - inicio_farm()).total_seconds() / 3600
    painel.append(("Total", [
        f"Relay: {CONTAINER} ({estado})",
        f"Personagens online: {total['online']} de {n}",
        f"!! {total['alertas']} aviso(s) de mochila" if total["alertas"] else "Mochilas: ok",
        f"Captchas: {total['captcha_ok']} aceitos, {total['captcha_erro']} recusados",
        f"Atualizado: {agora:%d/%m/%Y %H:%M:%S}",
        "",
        f"Fights desde {inicio_farm():%d/%m %H:%M}: {total['fights']}",
        f"Partidas por hora: {total['fights'] / horas:.1f}" if horas > 0.05 else "Partidas por hora: -",
        "",
    ] + ([f"{nome} × {q}" for nome, q in sorted(itens_total.items())] or ["Recompensas: nenhuma ainda"])))

    caixas = "\n".join(
        f'<section><h2>{html.escape(nome)}</h2><pre>{html.escape(chr(10).join(corpo))}</pre></section>'
        for nome, corpo in painel
    )
    (PASTA / "painel.html").write_text(f"""<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><meta http-equiv="refresh" content="{ATUALIZAR}"><title>Painel DDTank</title>
<style>
  body {{ margin: 0; padding: 24px; background: #0d1117; color: #d7dde5; font: 14px/1.55 "JetBrains Mono", Consolas, monospace; }}
  main {{ display: grid; grid-template-columns: repeat(3, 440px); gap: 24px; align-items: start; }}
  section {{ border: 1px solid #c9d1d9; padding: 14px 12px; }}
  section:last-child {{ grid-column: 3; grid-row: 1 / span 2; }}
  h2 {{ margin: 0 0 14px; font: inherit; }}
  pre {{ margin: 0; font: inherit; white-space: pre-wrap; }}
</style></head><body><main>
{caixas}
</main></body></html>
""", encoding="utf-8")
    print(f"[{agora:%H:%M:%S}] painel.html: {total['online']} de {n} online, {total['fights']} partidas", flush=True)


# Contagem do farm em farm.json: sobrevive a reinícios do grupo (o bot zera os contadores a cada
# login). Se os números do bot caírem, o grupo reiniciou e o que havia antes vai para "base".
FARM = PASTA / "farm.json"


def ler_farm():
    try:
        return json.loads(FARM.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {"inicio": datetime.now().astimezone().isoformat(), "personagens": {}}


def gravar_farm(farm):
    temporario = FARM.with_suffix(".novo")
    temporario.write_text(json.dumps(farm, ensure_ascii=False, indent=1), encoding="utf-8")
    temporario.replace(FARM)


def inicio_farm():
    return datetime.fromisoformat(ler_farm()["inicio"])


def somar(a, b):
    itens = Counter(a.get("itens", {}))
    itens.update(b.get("itens", {}))
    return {"runs": a.get("runs", 0) + b.get("runs", 0), "wins": a.get("wins", 0) + b.get("wins", 0),
            "losses": a.get("losses", 0) + b.get("losses", 0), "itens": dict(itens)}


def acumular(nome, atual):
    farm = ler_farm()
    p = farm["personagens"].setdefault(nome, {"base": {}, "ultimo": {}})
    ultimo = p["ultimo"]
    reiniciou = atual["runs"] < ultimo.get("runs", 0) or any(
        atual["itens"].get(item, 0) < q for item, q in ultimo.get("itens", {}).items())
    if reiniciou:
        p["base"] = somar(p["base"], ultimo)
    p["ultimo"] = atual
    gravar_farm(farm)
    return somar(p["base"], atual)


def farm_de(nome):
    p = ler_farm()["personagens"].get(nome, {})
    return somar(p.get("base", {}), p.get("ultimo", {}))


def zerar_farm():
    gravar_farm({"inicio": datetime.now().astimezone().isoformat(), "personagens": {}})


def relatorio():
    """Escreve relatorio-<data>.txt com partidas e drops de cada personagem desde o início do farm."""
    farm, agora = ler_farm(), datetime.now().astimezone()
    inicio = datetime.fromisoformat(farm["inicio"])
    horas = (agora - inicio).total_seconds() / 3600
    linhas, geral, partidas = [f"Farm do Formigueiro de {inicio:%d/%m/%Y %H:%M} a {agora:%d/%m/%Y %H:%M} ({horas:.1f} h)", ""], Counter(), 0
    for nome, p in farm["personagens"].items():
        t = somar(p.get("base", {}), p.get("ultimo", {}))
        partidas = max(partidas, t["runs"])
        linhas += [f"== {nome}: {t['runs']} partidas ({t['wins']} vitórias, {t['losses']} derrotas)"]
        linhas += [f"   {item} × {q}" for item, q in sorted(t["itens"].items())] + [""]
        geral.update(t["itens"])
    linhas += [f"== Total: {partidas} partidas ({partidas / horas:.1f} por hora)" if horas > 0 else "== Total"]
    linhas += [f"   {item} × {q}" for item, q in sorted(geral.items())]
    destino = PASTA / f"relatorio-{agora:%Y-%m-%d_%H%M}.txt"
    destino.write_text("\n".join(linhas) + "\n", encoding="utf-8")
    print("relatório:", destino)


def manter_acordado():
    """Pede ao Windows para não suspender enquanto este processo roda (a tela pode apagar).
    Não altera configurações de energia: o pedido acaba quando o painel fecha."""
    if sys.platform == "win32":
        import ctypes
        ES_CONTINUOUS, ES_SYSTEM_REQUIRED = 0x80000000, 0x00000001
        ctypes.windll.kernel32.SetThreadExecutionState(ES_CONTINUOUS | ES_SYSTEM_REQUIRED)
        print("PC mantido acordado enquanto o painel estiver aberto (a tela pode apagar).", flush=True)


if "--zerar" in sys.argv:
    zerar_farm()
    print("contagem do farm zerada")
elif "--relatorio" in sys.argv:
    try:
        main()
    except Exception as erro:  # noqa: BLE001 - o relatório usa o que já foi gravado
        print("painel não atualizado antes do relatório:", erro, flush=True)
    relatorio()
elif "--ao-vivo" in sys.argv:
    # Regera o painel sem parar; a página se recarrega sozinha a cada ATUALIZAR segundos.
    manter_acordado()
    while True:
        try:
            main()
        except Exception as erro:  # noqa: BLE001 - o painel não deve cair por uma leitura falha
            print("falha ao atualizar o painel:", erro, flush=True)
        time.sleep(ATUALIZAR)
else:
    main()
