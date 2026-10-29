"""Avalia o OcrLocal real (modelos ONNX do SDK local) em captchas sintéticos + imagem real 38.

Métrica principal: % de top-1 ERRADO (o bot enviaria um código errado). Também mostra
% top-1 certo, % sem candidato (o bot não envia nada), se o correto aparece em algum
candidato/leitura e o tempo médio por reconhecer(). "geral" é exatamente o que o bot
envia: o primeiro candidato de OcrLocal.reconhecer(). Por variante (original, cinza_2x,
mascara, cores), mostra o que aquela variante sozinha daria pelo maior score elegível
(diagnóstico; não é a regra do bot). A varredura de limiar usa a regra do módulo
(`decidir`, quando existe; senão maior score, como no código antigo).

Para comparar com outra versão do adaptador sem alterar src/:
    git show HEAD:src/verificacao_ocr.py > logs/verificacao_ocr_head.py
    python -B tools/avaliar_ocr.py --modulo-ocr logs/verificacao_ocr_head.py ...

Uso:
    python -B tools/avaliar_ocr.py --n 300 --semente 1 --json logs/avaliacao_ocr_base.json

Offline: não captura tela, não importa drivers. Sai com 0 quando roda sem erro de
infraestrutura (modelos ausentes/falha de inferência => 1). Acerto em sintético não
comprova acerto no jogo.
"""
from __future__ import annotations

import argparse
import importlib.util
from collections import Counter, defaultdict
import json
from pathlib import Path
import re
import sys
import time
from types import SimpleNamespace

import cv2
import numpy as np

RAIZ = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(RAIZ))

from tools.gerar_captchas import NIVEIS, gerar  # noqa: E402

IMAGEM_REAL = RAIZ / "docs" / "especificacao" / "imagens" / "38_codigo_verificacao.png"
ROTULO_REAL = "6723"
VARIANTES = ("geral", "original", "cinza_2x", "mascara", "cores")
LIMIARES_VARREDURA = (0.5, 0.8, 0.9, 0.95, 0.97, 0.98, 0.99, 0.995)


def _elegivel(leitura, limiar):
    t, c = leitura["texto"], leitura["confianca"]
    return (isinstance(t, str) and re.fullmatch(r"[0-9]{4}", t) is not None
            and np.isfinite(c) and limiar <= c <= 1.0)


def top1(leituras, limiar, variante="geral"):
    """Maior confiança entre leituras elegíveis (regra do bot até o commit 4e5a9de)."""
    escolhidas = [l for l in leituras if _elegivel(l, limiar)
                  and (variante == "geral" or l["variante"] == variante)]
    if not escolhidas:
        return None
    return max(escolhidas, key=lambda l: l["confianca"])


def _levenshtein(a, b):
    d = list(range(len(b) + 1))
    for i, ca in enumerate(a, 1):
        anterior, d[0] = d[0], i
        for j, cb in enumerate(b, 1):
            anterior, d[j] = d[j], min(d[j] + 1, d[j - 1] + 1, anterior + (ca != cb))
    return d[-1]


def classificar_erro(rotulo, lido):
    """Categoria do erro de um top-1 de 4 dígitos diferente do rótulo."""
    if sorted(rotulo) == sorted(lido):
        return "ordem_trocada"
    hamming = sum(a != b for a, b in zip(rotulo, lido))
    if hamming == 1:
        return "um_digito_confundido"
    if _levenshtein(rotulo, lido) < hamming:
        return "digito_faltando_e_extra"  # ex.: 1234 -> 2345 (deslocamento)
    return "varios_digitos"


def _vazio():
    return {"n": 0, "certo": 0, "errado": 0, "sem_candidato": 0}


def _decisor_do_modulo(ocr):
    """`decidir(leituras, limiar)` do módulo do OCR; sem ela, maior score (código antigo)."""
    decidir = getattr(sys.modules.get(type(ocr).__module__), "decidir", None)
    if decidir is None:
        def decidir(leituras, limiar):
            t = top1(leituras, limiar)
            return [] if t is None else [SimpleNamespace(**t)]
    return decidir


def avaliar(ocr, amostras, limiar, progresso=None, decisor=None):
    """amostras: iterável de (nivel, id, rotulo, imagem, caso). Devolve dict com métricas.

    O top-1 "geral" é o primeiro candidato devolvido por ocr.reconhecer(); `decisor`
    (padrão: `decidir` do módulo do OCR) só é usado para a varredura de limiar.
    """
    decisor = decisor or _decisor_do_modulo(ocr)
    metricas = defaultdict(lambda: {v: _vazio() for v in VARIANTES})
    extras = defaultdict(lambda: {"correto_em_candidato": 0, "correto_em_leitura": 0,
                                  "tempos": [], "erros": Counter(), "confusoes": Counter(),
                                  "conf_certo": [], "conf_errado": [],
                                  "varredura": {str(t): _vazio() for t in LIMIARES_VARREDURA},
                                  "por_caso": defaultdict(_vazio), "exemplos_erro": []})
    for k, (nivel, ident, rotulo, imagem, caso) in enumerate(amostras):
        inicio = time.perf_counter()
        candidatos = ocr.reconhecer(imagem, limiar=limiar)
        duracao = time.perf_counter() - inicio
        leituras = list(getattr(ocr, "ultimas_leituras", []))
        e = extras[nivel]
        e["tempos"].append(duracao)
        e["correto_em_candidato"] += any(c.texto == rotulo for c in candidatos)
        e["correto_em_leitura"] += any(l["texto"] == rotulo for l in leituras)
        enviado = candidatos[0] if candidatos else None
        t = None if enviado is None else {"texto": enviado.texto, "confianca": enviado.confianca,
                                         "modelo": enviado.modelo, "variante": enviado.variante}
        for variante in VARIANTES:
            m = metricas[nivel][variante]
            m["n"] += 1
            tv = t if variante == "geral" else top1(leituras, limiar, variante)
            if tv is None:
                m["sem_candidato"] += 1
            elif tv["texto"] == rotulo:
                m["certo"] += 1
            else:
                m["errado"] += 1
        if caso:
            pc = e["por_caso"][caso]
            pc["n"] += 1
            pc["sem_candidato" if t is None else ("certo" if t["texto"] == rotulo else "errado")] += 1
        if t is not None:
            (e["conf_certo"] if t["texto"] == rotulo else e["conf_errado"]).append(t["confianca"])
            if t["texto"] != rotulo:
                cat = classificar_erro(rotulo, t["texto"])
                e["erros"][cat] += 1
                if cat == "um_digito_confundido":
                    a, b = next((a, b) for a, b in zip(rotulo, t["texto"]) if a != b)
                    e["confusoes"][f"{a}->{b}"] += 1
                if len(e["exemplos_erro"]) < 40:
                    e["exemplos_erro"].append({"id": ident, "rotulo": rotulo, "lido": t["texto"],
                                               "confianca": round(t["confianca"], 4),
                                               "modelo": t["modelo"], "variante": t["variante"],
                                               "categoria": cat, "caso": caso})
        for lim in LIMIARES_VARREDURA:
            v = e["varredura"][str(lim)]
            v["n"] += 1
            tt = decisor(leituras, lim)
            v["sem_candidato" if not tt else ("certo" if tt[0].texto == rotulo else "errado")] += 1
        if progresso:
            progresso(k, nivel)
    return _resumir(metricas, extras)


def _pct(a, n):
    return round(100.0 * a / n, 2) if n else 0.0


def _resumir(metricas, extras):
    saida = {}
    for nivel, por_var in metricas.items():
        e = extras[nivel]
        n = por_var["geral"]["n"]
        saida[nivel] = {
            "n": n,
            "variantes": {v: {**m, "pct_certo": _pct(m["certo"], m["n"]),
                              "pct_errado": _pct(m["errado"], m["n"]),
                              "pct_sem_candidato": _pct(m["sem_candidato"], m["n"])}
                          for v, m in por_var.items()},
            "pct_correto_em_candidato": _pct(e["correto_em_candidato"], n),
            "pct_correto_em_leitura": _pct(e["correto_em_leitura"], n),
            "tempo_medio_s": round(float(np.mean(e["tempos"])), 4) if e["tempos"] else None,
            "erros_por_categoria": dict(e["erros"].most_common()),
            "confusoes_um_digito": dict(e["confusoes"].most_common(15)),
            "confianca_top1": {
                "certo": _quantis(e["conf_certo"]), "errado": _quantis(e["conf_errado"])},
            "varredura_limiar_geral": {lim: {**v, "pct_certo": _pct(v["certo"], v["n"]),
                                             "pct_errado": _pct(v["errado"], v["n"])}
                                       for lim, v in e["varredura"].items()},
            "por_caso": {c: {**v, "pct_errado": _pct(v["errado"], v["n"])}
                         for c, v in e["por_caso"].items()},
            "exemplos_erro": e["exemplos_erro"],
        }
    return saida


def _quantis(valores):
    if not valores:
        return {"n": 0}
    q = np.quantile(valores, [0, 0.1, 0.5, 0.9, 1])
    return {"n": len(valores), "min": round(float(q[0]), 4), "p10": round(float(q[1]), 4),
            "mediana": round(float(q[2]), 4), "p90": round(float(q[3]), 4),
            "max": round(float(q[4]), 4)}


def amostras_sinteticas(semente, niveis, n):
    for nivel in niveis:
        for i in range(n):
            c = gerar(semente, nivel, i)
            yield nivel, f"{nivel}/{i}", c.rotulo, c.imagem, c.caso


def amostra_real():
    from src.verificacao_visao import VisaoVerificacao
    frame = cv2.imread(str(IMAGEM_REAL))
    quadro = VisaoVerificacao().analisar(frame)
    if quadro is None:
        raise RuntimeError("quadro de verificação não encontrado na imagem 38")
    return "real", "real/38", ROTULO_REAL, quadro.codigo, ""


def imprimir(resultado, limiar):
    print(f"\nLimiar {limiar}. Taxa de ERRO = top-1 elegível diferente do rótulo (o bot enviaria errado).")
    cab = f"{'nível':<12}{'variante':<10}{'n':>5}{'certo%':>9}{'ERRADO%':>9}{'sem%':>8}"
    print(cab)
    print("-" * len(cab))
    for nivel, r in resultado.items():
        for v in VARIANTES:
            m = r["variantes"][v]
            print(f"{nivel:<12}{v:<10}{m['n']:>5}{m['pct_certo']:>9.1f}{m['pct_errado']:>9.1f}"
                  f"{m['pct_sem_candidato']:>8.1f}")
        print(f"{'':<12}correto em algum candidato: {r['pct_correto_em_candidato']:.1f}% | "
              f"em alguma leitura bruta: {r['pct_correto_em_leitura']:.1f}% | "
              f"tempo médio: {r['tempo_medio_s']:.3f} s")
        if r["erros_por_categoria"]:
            print(f"{'':<12}erros: {r['erros_por_categoria']}  confusões: {r['confusoes_um_digito']}")
        if r["por_caso"]:
            print(f"{'':<12}por caso: " + ", ".join(
                f"{c}: errado {v['pct_errado']:.0f}% certo {v['certo']}/{v['n']}"
                for c, v in r["por_caso"].items()))
        varr = r["varredura_limiar_geral"]
        print(f"{'':<12}limiar->(certo%/ERRADO%): " + " ".join(
            f"{lim}:{v['pct_certo']:.0f}/{v['pct_errado']:.1f}" for lim, v in varr.items()))
        ct = r["confianca_top1"]
        print(f"{'':<12}confiança top-1 certo {ct['certo']} | errado {ct['errado']}")


def carregar_ocr(caminho=None):
    if not caminho:
        from src.verificacao_ocr import OcrLocal
        return OcrLocal
    nome = "ocr_alternativo_" + Path(caminho).stem
    spec = importlib.util.spec_from_file_location(nome, caminho)
    modulo = importlib.util.module_from_spec(spec)
    sys.modules[nome] = modulo
    spec.loader.exec_module(modulo)
    return modulo.OcrLocal


def main(argv=None):
    ap = argparse.ArgumentParser(description="Avalia o OCR local em captchas sintéticos e no real.")
    ap.add_argument("--n", type=int, default=300, help="amostras por nível")
    ap.add_argument("--semente", type=int, default=1)
    ap.add_argument("--niveis", default=",".join(NIVEIS))
    ap.add_argument("--limiar", type=float, default=0.90)
    ap.add_argument("--json", help="caminho do relatório JSON")
    ap.add_argument("--sem-real", action="store_true", help="não inclui a imagem 38")
    ap.add_argument("--modulo-ocr", help="arquivo .py alternativo com OcrLocal (ex.: versão do HEAD)")
    args = ap.parse_args(argv)
    niveis = [n for n in args.niveis.split(",") if n]

    OcrLocal = carregar_ocr(args.modulo_ocr)
    try:
        t0 = time.perf_counter()
        ocr = OcrLocal()
        carga = time.perf_counter() - t0
        amostras = list(amostras_sinteticas(args.semente, niveis, args.n))
        if not args.sem_real:
            amostras.append(amostra_real())
        total = len(amostras)

        def progresso(k, nivel):
            if (k + 1) % 50 == 0 or k + 1 == total:
                print(f"  {k + 1}/{total} ({nivel})", file=sys.stderr, flush=True)

        t1 = time.perf_counter()
        resultado = avaliar(ocr, amostras, args.limiar, progresso)
        duracao = time.perf_counter() - t1
    except RuntimeError as exc:  # ErroInfraestrutura (de qualquer versão do módulo)
        print(f"Erro de infraestrutura: {exc}", file=sys.stderr)
        return 1

    imprimir(resultado, args.limiar)
    print(f"\nCarga dos modelos {carga:.1f} s; avaliação {duracao:.1f} s para {total} imagens.")
    if args.json:
        import ddddocr
        import subprocess
        try:
            commit = subprocess.run(["git", "rev-parse", "--short", "HEAD"], cwd=RAIZ,
                                    capture_output=True, text=True).stdout.strip()
        except OSError:
            commit = ""
        relatorio = {"commit": commit, "python": sys.version.split()[0],
                     "ddddocr": getattr(ddddocr, "__version__", "?"),
                     "modulo_ocr": args.modulo_ocr or "src/verificacao_ocr.py",
                     "opencv": cv2.__version__, "semente": args.semente, "n_por_nivel": args.n,
                     "limiar": args.limiar, "carga_modelos_s": round(carga, 2),
                     "duracao_s": round(duracao, 1), "niveis": resultado}
        caminho = Path(args.json)
        caminho.parent.mkdir(parents=True, exist_ok=True)
        caminho.write_text(json.dumps(relatorio, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"JSON: {caminho}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
