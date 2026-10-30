"""Grava uma conta jogando à mão para ensinar uma instância nova ao bot (só observa).

Uso: python tools/gravar_sessao.py [hwnd]   (sem número: clique na janela do jogo ao iniciar)
Ctrl+C para parar.

Não clica nem aperta nada: lê a janela por PrintWindow e consulta o estado das teclas e do
mouse (GetAsyncKeyState), só enquanto a janela do jogo estiver na frente.
Em logs/gravacao/AAAAMMDD_HHMMSS/ grava:
- quadros/  : print quando a tela muda (no máximo 1 por segundo) e em cada clique/Espaço;
- eventos.csv: teclas, cliques (posição na janela), telas reconhecidas e cada tiro;
- resumo.txt: um resumo por turno (itens, andar, ângulo e força do tiro).
"""
import csv
import sys
import threading
import time
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path

import cv2
import numpy as np

RAIZ = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(RAIZ))
from src import batalha, config, telas  # noqa: E402

# Teclas observadas (código virtual → nome). Letras e F1–F12 também, para não perder atalhos.
TECLAS = {0x20: "espaco", 0x25: "esquerda", 0x26: "cima", 0x27: "direita", 0x28: "baixo",
          0x09: "tab", 0x0D: "enter", 0x1B: "esc"}
TECLAS.update({0x30 + i: str(i) for i in range(10)})
TECLAS.update({0x41 + i: chr(0x41 + i) for i in range(26)})
TECLAS.update({0x70 + i: f"F{i + 1}" for i in range(12)})
MOUSE = {0x01: "clique_esq", 0x02: "clique_dir"}

INTERVALO_TECLAS = 0.01
INTERVALO_QUADROS = 1.0
MUDANCA_MIN = 4.0  # diferença média (0–255) da miniatura para considerar que a tela mudou


def mudou(antes, depois, minimo=MUDANCA_MIN):
    """A tela mudou de verdade? Compara miniaturas em cinza (ignora ruído pequeno)."""
    if antes is None or antes.shape != depois.shape:
        return True
    a = cv2.resize(cv2.cvtColor(antes, cv2.COLOR_BGR2GRAY), (160, 90), interpolation=cv2.INTER_AREA)
    b = cv2.resize(cv2.cvtColor(depois, cv2.COLOR_BGR2GRAY), (160, 90), interpolation=cv2.INTER_AREA)
    return float(np.abs(a.astype(int) - b.astype(int)).mean()) >= minimo


@dataclass
class Turno:
    """O que o jogador fez numa vez: itens, passos, ajuste de ângulo e tiros."""
    inicio: float
    teclas: list = field(default_factory=list)  # (nome, segundos segurada)
    tiros: list = field(default_factory=list)  # dicts de ler_tiro

    def resumo(self):
        andar = {"esquerda": 0.0, "direita": 0.0}
        angulo = {"cima": 0, "baixo": 0}
        itens, outras = [], []
        for nome, segundos in self.teclas:
            if nome in andar:
                andar[nome] += segundos
            elif nome in angulo:
                angulo[nome] += 1
            elif nome.isdigit():
                itens.append(nome)
            elif nome != "espaco":
                outras.append(nome)
        partes = [f"vez de {time.time() - self.inicio:.0f}s"]
        if itens:
            partes.append("itens " + " ".join(itens))
        for lado, s in andar.items():
            if s:
                partes.append(f"andou {lado} {s:.1f}s")
        if angulo["cima"] or angulo["baixo"]:
            partes.append(f"ângulo ↑{angulo['cima']} ↓{angulo['baixo']}")
        if outras:
            partes.append("outras " + " ".join(outras))
        for t in self.tiros:
            partes.append("tiro " + descrever_tiro(t))
        if not self.tiros:
            partes.append("sem tiro")
        return "; ".join(partes)


def ler_tiro(frame_soltou, barra_vazia, segurou):
    """Ângulo, lado e força do tiro no print tirado ao soltar o Espaço."""
    leitura = batalha.ler(frame_soltou)
    tiro = {"segurou_s": round(segurou, 2), "angulo": None, "lado": None, "forca": None}
    if leitura is None:
        return tiro
    ancora = leitura.ancora
    angulo = batalha.ler_angulo(frame_soltou, ancora)
    tiro["angulo"] = None if angulo is None else round(angulo)
    lado = batalha.lado_virado(frame_soltou, ancora)
    tiro["lado"] = {1: "direita", -1: "esquerda"}.get(lado)
    if barra_vazia is not None:
        tiro["forca"] = round(batalha.forca_na_barra(barra_vazia, recorte_barra(frame_soltou, ancora)))
    return tiro


def descrever_tiro(t):
    angulo = "?" if t["angulo"] is None else f"{t['angulo']}°"
    forca = "?" if t["forca"] is None else str(t["forca"])
    return f"ângulo {angulo}, força {forca}, {t['lado'] or 'lado ?'} (Espaço {t['segurou_s']}s)"


def recorte_barra(frame, ancora):
    """A faixa da barra de força medida pelo bot (mesma região de Batalha.atirar)."""
    ax, ay = ancora
    y = ay + batalha.BARRA_Y - 2
    return frame[y:y + 5, ax + batalha.BARRA_X0:ax + batalha.BARRA_X100 + 1]


class Gravacao:
    def __init__(self, hwnd, pasta):
        import win32gui
        from src.drivers.janela import DriverJanela
        self.win32gui = win32gui
        self.hwnd = hwnd
        self.janela = DriverJanela(hwnd)
        self.pasta = pasta
        (pasta / "quadros").mkdir(parents=True, exist_ok=True)
        self.inicio = time.time()
        self.trava = threading.Lock()  # eventos.csv e o turno atual são usados pelas 2 threads
        self.arq_eventos = open(pasta / "eventos.csv", "w", newline="", encoding="utf-8")
        self.eventos = csv.writer(self.arq_eventos)
        self.eventos.writerow(["t", "tipo", "detalhe", "x", "y", "quadro"])
        self.n_quadro = 0
        self.ultimo_quadro = None
        self.ultimo_salvo = 0.0
        self.tela = None
        self.minha_vez = False
        self.turno = None
        self.barra_vazia = None  # faixa da barra de força antes do Espaço (referência)
        self.parar = threading.Event()

    # ---- registro ----
    def evento(self, tipo, detalhe="", x="", y="", quadro=""):
        with self.trava:
            self.eventos.writerow([f"{time.time() - self.inicio:.2f}", tipo, detalhe, x, y, quadro])
            self.arq_eventos.flush()

    def avisar(self, msg):
        linha = f"[{datetime.now():%H:%M:%S}] {msg}"
        print(linha, flush=True)
        with open(self.pasta / "resumo.txt", "a", encoding="utf-8") as f:
            f.write(linha + "\n")

    def salvar(self, frame, motivo):
        with self.trava:
            self.n_quadro += 1
            nome = f"{self.n_quadro:05d}_{time.time() - self.inicio:07.1f}s_{motivo}.png"
        cv2.imwrite(str(self.pasta / "quadros" / nome), frame)
        return nome

    def na_frente(self):
        return self.win32gui.GetForegroundWindow() == self.hwnd

    # ---- teclado e mouse (thread própria, só lê o estado) ----
    def observar_entrada(self):
        import win32api
        apertada = {}  # código → quando desceu
        while not self.parar.is_set():
            frente = self.na_frente()
            for codigo in (*TECLAS, *MOUSE):
                baixo = bool(win32api.GetAsyncKeyState(codigo) & 0x8000) and frente
                if baixo and codigo not in apertada:
                    apertada[codigo] = time.time()
                    self.desceu(codigo, win32api)
                elif not baixo and codigo in apertada:
                    self.subiu(codigo, time.time() - apertada.pop(codigo))
            time.sleep(INTERVALO_TECLAS)

    def desceu(self, codigo, win32api):
        if codigo in MOUSE:
            x, y = self.win32gui.ScreenToClient(self.hwnd, win32api.GetCursorPos())
            # Print no clique: mostra o que estava na tela (para recortar o template do alvo).
            quadro = self.salvar(self.janela.capturar(), MOUSE[codigo])
            self.evento(MOUSE[codigo], "", x, y, quadro)
            return
        if codigo == 0x20:
            frame = self.janela.capturar()
            leitura = batalha.ler(frame)
            angulo = batalha.ler_angulo(frame, leitura.ancora) if leitura else None
            quadro = self.salvar(frame, "espaco_desce")
            self.evento("tecla_desce", "espaco", quadro=quadro,
                        x="" if angulo is None else f"ângulo {angulo:.0f}")
            return
        self.evento("tecla_desce", TECLAS[codigo])

    def subiu(self, codigo, segurou):
        if codigo in MOUSE:
            return
        nome = TECLAS[codigo]
        if codigo == 0x20:
            frame = self.janela.capturar()
            quadro = self.salvar(frame, "espaco_solta")
            tiro = ler_tiro(frame, self.barra_vazia, segurou)
            self.evento("tiro", descrever_tiro(tiro), quadro=quadro)
            self.avisar("tiro: " + descrever_tiro(tiro))
            with self.trava:
                if self.turno is not None:
                    self.turno.tiros.append(tiro)
        self.evento("tecla_sobe", nome, f"{segurou:.2f}s")
        with self.trava:
            if self.turno is not None:
                self.turno.teclas.append((nome, segurou))

    # ---- telas (thread principal) ----
    def olhar(self):
        frame = self.janela.capturar()
        leitura = batalha.ler(frame)
        if leitura is not None:
            nome = "BATALHA"
            if not leitura.minha_vez or self.barra_vazia is None:
                self.barra_vazia = recorte_barra(frame, leitura.ancora)
            self.vez(leitura.minha_vez, frame)
        else:
            tela = telas.identificar(frame, config.LIMIAR, ignorar=("SEM_FOCO",))
            nome = tela.nome if tela else "desconhecida"
            self.vez(False, frame)
        if nome != self.tela:
            self.tela = nome
            quadro = self.salvar(frame, f"tela_{nome}")
            self.evento("tela", nome, quadro=quadro)
            self.avisar(f"tela: {nome}")
        elif (time.time() - self.ultimo_salvo >= INTERVALO_QUADROS
              and mudou(self.ultimo_quadro, frame)):
            self.salvar(frame, nome)
        else:
            return
        self.ultimo_quadro, self.ultimo_salvo = frame, time.time()

    def vez(self, minha, frame):
        if minha == self.minha_vez:
            return
        self.minha_vez = minha
        if minha:
            with self.trava:
                self.turno = Turno(time.time())
            quadro = self.salvar(frame, "vez_comeca")
            self.evento("vez", "começou", quadro=quadro)
            self.avisar("minha vez")
        else:
            with self.trava:
                turno, self.turno = self.turno, None
            self.evento("vez", "acabou")
            if turno is not None:
                self.avisar("turno: " + turno.resumo())

    def rodar(self):
        self.avisar(f"gravando a janela {self.hwnd} em {self.pasta} (Ctrl+C para parar)")
        entrada = threading.Thread(target=self.observar_entrada, daemon=True)
        entrada.start()
        try:
            while True:
                self.olhar()
                time.sleep(0.2)
        except KeyboardInterrupt:
            self.avisar("gravação parada")
        finally:
            self.parar.set()
            entrada.join(1)
            self.arq_eventos.close()


def main():
    from src.drivers.conta import contas_abertas, esperar_clique_na_principal
    from src.drivers.tela import ativar_dpi
    ativar_dpi()  # coordenadas de clique iguais às do bot
    hwnds = contas_abertas()
    hwnd = int(sys.argv[1]) if len(sys.argv) > 1 else esperar_clique_na_principal(hwnds)
    if hwnd is None:
        print("Nenhuma janela do jogo escolhida.")
        return 1
    pasta = RAIZ / "logs" / "gravacao" / f"{datetime.now():%Y%m%d_%H%M%S}"
    Gravacao(hwnd, pasta).rodar()
    return 0


if __name__ == "__main__":
    sys.exit(main())
