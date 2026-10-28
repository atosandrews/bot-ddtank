"""Testa o reconhecimento de telas nos prints da especificação, sem abrir o jogo.
Uso: python tools/testar_prints.py
"""
import sys
from pathlib import Path

import cv2

RAIZ = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(RAIZ))
from src import config, telas, vision

PRINTS = RAIZ / "docs" / "especificacao" / "imagens"
ESPERADO = {
    "01_tela_selecao_canal.png": "SELECAO_CANAL",
    "03_tela_aluno_recomendado.png": "ALUNO_RECOMENDADO",
    "05_tela_lobby_principal.png": "LOBBY",
    "07_tela_cais_expedicao.png": "CAIS_EXPEDICAO",
    "09_tela_sala.webp": "SALA_SEM_INSTANCIA",
    "16_tela_sala_janela_real.png": "SALA_SEM_INSTANCIA",
    "17_tela_configuracao_sala.png": "SELECAO_INSTANCIA",
    "18_configuracao_sala_janela_real.png": "SELECAO_INSTANCIA",
    # Recortes parciais (não há print da tela inteira): só confirmam o reconhecimento.
    "14_botoes_convide_troque_inicio.png": "SALA_PRONTA",
    "15_popup_lembrete.png": "LEMBRETE",
}

falhas = 0
for arquivo, esperado in ESPERADO.items():
    frame = cv2.imread(str(PRINTS / arquivo))
    print(f"\n{arquivo} (esperado: {esperado})")
    for tela in telas.TELAS:
        _, conf = vision.procurar(tela.reconhecer, frame)
        print(f"  {tela.nome:<18} confiança {conf:.2f}")
    tela = telas.identificar(frame, config.LIMIAR)
    nome = tela.nome if tela else None
    ok = nome == esperado
    print(f"  -> reconhecida: {nome}  {'OK' if ok else 'FALHOU'}")
    for clique in tela.cliques if tela else []:
        if clique.opcional:
            continue
        pos, conf = vision.procurar(clique.alvo, frame)
        ok = ok and conf >= config.LIMIAR
        print(f"     alvo {clique.alvo.name:<26} {pos} confiança {conf:.2f}")
    falhas += not ok

print("\nTudo OK" if not falhas else f"\n{falhas} tela(s) com falha")
sys.exit(1 if falhas else 0)
