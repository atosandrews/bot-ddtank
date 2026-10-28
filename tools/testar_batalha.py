"""Mostra o que o bot lê da batalha e a força que usaria, SEM apertar nada.
Uso:
  python tools/testar_batalha.py                       (print da janela do jogo, ao vivo)
  python tools/testar_batalha.py caminho/do/print.png  (um print salvo)
Salva logs/teste_batalha.png com as marcações: verde = eu, vermelho = inimigos, amarelo = alvo.
"""
import sys
from pathlib import Path

import cv2

RAIZ = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(RAIZ))
from src import batalha, config

if len(sys.argv) > 1:
    frame = cv2.imread(sys.argv[1])
else:
    from src.drivers import DriverTela
    frame = DriverTela.da_janela(config.TITULO_JANELA).capturar()

leitura = batalha.ler(frame)
if leitura is None:
    print("Não é a tela de batalha (não achei o rótulo 'Força').")
    sys.exit(1)

print("Rótulo 'Força' em:", leitura.ancora)
print("Minha vez (PASS na tela):", leitura.minha_vez)
print("Estágio:", leitura.estagio)
print("Ângulo (agulha):", None if (a := batalha.ler_angulo(frame, leitura.ancora)) is None else round(a, 1))
estrategia = config.ESTRATEGIAS[leitura.estagio]
print("Estratégia:", estrategia)
print("Eu:", leitura.eu)
print("Inimigos:", leitura.inimigos)
ultimo = batalha.marcador(frame, leitura.ancora)
print("Força do último tiro (marcador):", None if ultimo is None else round(ultimo))

marcado = frame.copy()
if leitura.eu:
    cv2.circle(marcado, leitura.eu, 12, (0, 255, 0), 3)
for p in leitura.inimigos:
    cv2.circle(marcado, p, 10, (0, 0, 255), 2)
if leitura.eu and leitura.inimigos:
    ex = leitura.eu[0]
    alvo = min(leitura.inimigos, key=lambda p: abs(p[0] - ex))
    dist = abs(alvo[0] - ex) / config.PX_POR_DISTANCIA
    forca = estrategia["forca"] or batalha.forca_para(estrategia["angulo"], dist)
    print(f"Inimigo mais perto: {alvo}, distância {dist:.2f} -> força {forca:.0f}")
    cv2.line(marcado, leitura.eu, alvo, (0, 255, 255), 2)
    cv2.circle(marcado, alvo, 14, (0, 255, 255), 3)
ax, ay = leitura.ancora
cv2.line(marcado, (ax + batalha.BARRA_X0, ay + batalha.BARRA_Y), (ax + batalha.BARRA_X100, ay + batalha.BARRA_Y), (255, 0, 255), 1)
(RAIZ / "logs").mkdir(exist_ok=True)
cv2.imwrite(str(RAIZ / "logs" / "teste_batalha.png"), marcado)
print("Print marcado em logs/teste_batalha.png")
