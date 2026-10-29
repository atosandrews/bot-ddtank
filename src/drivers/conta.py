"""Fase 3: uma janela do jogo por conta.

Vê a janela mesmo coberta (PrintWindow) e, para clicar ou apertar teclas, traz a janela
para frente e usa o mouse/teclado reais, como o DriverTela. Assim um bot só controla
várias contas: na batalha só uma joga por vez, e na sala os cliques são rápidos.
"""
import time

import win32gui

from .. import config, vision
from .janela import DriverJanela
from .tela import DriverTela, trazer_para_frente

AVISO_FOCO = None  # carregado na 1ª vez (evita import circular com telas)


class DriverConta(DriverTela):
    def __init__(self, hwnd, nome):
        super().__init__(hwnd=hwnd)
        self.nome = nome
        self._janela = DriverJanela(hwnd)
        self._conferido_em = 0.0  # última vez que confirmei: na frente e sem o aviso

    def na_frente(self):
        return win32gui.GetForegroundWindow() == self.hwnd

    def capturar(self, sub=None):
        # Na frente: print da tela (rápido, serve para a barra de força).
        # Coberta: PrintWindow (mais lento, mas enxerga através das outras janelas).
        if self.na_frente():
            return super().capturar(sub)
        frame = self._janela.capturar()
        if sub:
            x, y, w, h = sub
            return frame[y:y + h, x:x + w]
        return frame

    def com_foco(self):
        """Traz esta conta para frente e, se o jogo mostrar 'Clique no jogo', clica nele."""
        global AVISO_FOCO
        if AVISO_FOCO is None:
            from ..telas import IMG
            AVISO_FOCO = IMG / "clique_no_jogo.png"
        # Já na frente e conferida há pouco: não procura o aviso de novo (cada tecla passa
        # por aqui, e procurar na tela inteira atrasa o turno).
        if self.na_frente() and time.time() - self._conferido_em < 2:
            return True
        if not self.na_frente():
            trazer_para_frente(self.hwnd)
            self._atualizar_regiao()
            if not self.na_frente():
                # Ficou por cima, mas o Windows não passou o foco: um clique nela passa.
                # Só clico se for mesmo esta janela que está debaixo do ponto.
                pos = vision.achar(AVISO_FOCO, self._janela.capturar(), config.LIMIAR)
                x, y = pos or (self.regiao["width"] // 2, self.regiao["height"] // 2)
                ponto = (self.regiao["left"] + x, self.regiao["top"] + y)
                if win32gui.GetAncestor(win32gui.WindowFromPoint(ponto), 2) == self.hwnd:
                    self._clicar_direto(x, y, vezes=2)
                    time.sleep(0.4)
            if not self.na_frente():
                print(f"Aviso: não consegui trazer a janela de {self.nome} para frente.", flush=True)
                return False
        self._atualizar_regiao()
        for _ in range(3):
            pos = vision.achar(AVISO_FOCO, super().capturar(), config.LIMIAR)
            if pos is None:
                self._conferido_em = time.time()
                return True
            # O aviso 'Clique no jogo' precisa de duplo clique.
            self._clicar_direto(*pos, vezes=2)
            time.sleep(0.4)
        return True

    def ativar_com_clique(self):
        """Traz a janela para frente e dá 1 clique nela (no aviso 'Clique no jogo', se houver,
        ou no meio da tela). Feito a cada turno, antes de jogar: só assim o jogo aceita as teclas."""
        global AVISO_FOCO
        if AVISO_FOCO is None:
            from ..telas import IMG
            AVISO_FOCO = IMG / "clique_no_jogo.png"
        for tentativa in range(4):
            trazer_para_frente(self.hwnd)
            self._atualizar_regiao()
            pos = vision.achar(AVISO_FOCO, self._janela.capturar(), config.LIMIAR)
            x, y = pos or (self.regiao["width"] // 2, self.regiao["height"] // 2)
            ponto = (self.regiao["left"] + x, self.regiao["top"] + y)
            if win32gui.GetAncestor(win32gui.WindowFromPoint(ponto), 2) != self.hwnd:
                time.sleep(0.3)  # outra janela por cima do ponto: não clica nela
                continue
            # 1º: um clique; se o aviso continuar, duplo clique.
            self._clicar_direto(x, y, vezes=1 if tentativa == 0 else 2)
            time.sleep(0.4)
            tela = DriverTela.capturar(self) if self.na_frente() else self._janela.capturar()
            if self.na_frente() and vision.achar(AVISO_FOCO, tela, config.LIMIAR) is None:
                self._conferido_em = time.time()
                return True
        print(f"Aviso: {self.nome} continua sem foco depois dos cliques.", flush=True)
        return False

    def _clicar_direto(self, x, y, vezes=1):
        import pyautogui
        ox, oy = self.regiao["left"], self.regiao["top"]
        pyautogui.moveTo(ox + x, oy + y, duration=0.1)
        pyautogui.click(clicks=vezes, interval=0.1)


def organizar_janelas(hwnds, largura=1366, altura=780):
    """Espalha as janelas no monitor da primeira, sem nenhuma ficar totalmente coberta.

    O cliente para de desenhar a janela que está toda escondida atrás de outra (ela congela:
    o convite não aparece, a vez não é vista). Com um canto de cada uma à mostra, todas
    continuam desenhando e o PrintWindow vê tudo.
    """
    import win32api
    import win32con
    monitor = win32api.MonitorFromWindow(hwnds[0], win32con.MONITOR_DEFAULTTONEAREST)
    esq, topo, dir_, base = win32api.GetMonitorInfo(monitor)["Work"]
    dx = max(dir_ - esq - largura, 0)
    dy = max(base - topo - altura, 0)
    # Até 4 janelas: uma em cada canto do monitor.
    cantos = [(0, 0), (dx, dy), (dx, 0), (0, dy)]
    for i, hwnd in enumerate(hwnds):
        x, y = cantos[i % 4]
        win32gui.ShowWindow(hwnd, win32con.SW_RESTORE)
        win32gui.SetWindowPos(hwnd, 0, esq + x, topo + y, largura, altura,
                              win32con.SWP_NOZORDER | win32con.SWP_NOACTIVATE)


def contas_abertas():
    """Janelas do jogo abertas (título exato)."""
    from .tela import listar_janelas
    return [hwnd for hwnd, titulo in listar_janelas() if titulo == config.TITULO_JANELA]


def esperar_clique_na_principal(hwnds, segundos=60):
    """Espera o usuário clicar numa das janelas do jogo e devolve essa (a principal)."""
    print("Clique na janela da CONTA PRINCIPAL (a dona da sala)...", flush=True)
    limite = time.time() + segundos
    while time.time() < limite:
        frente = win32gui.GetForegroundWindow()
        if frente in hwnds:
            return frente
        time.sleep(0.2)
    return None
