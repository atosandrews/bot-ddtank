"""Fase 1: captura a tela (ou uma região) e usa o mouse real."""
import ctypes
import random
import time

import cv2
import mss
import numpy as np
import pyautogui

# Pausa automática do pyautogui depois de cada comando (o padrão de 0.1 s atrasa a força).
pyautogui.PAUSE = 0.03


class DriverTela:
    def __init__(self, regiao=None, hwnd=None):
        # regiao = {"left": x, "top": y, "width": w, "height": h}
        # Com hwnd, a região acompanha a área interna da janela a cada captura.
        self.regiao = regiao
        self.hwnd = hwnd
        self._sct = None

    def _atualizar_regiao(self):
        import win32gui
        l, t, r, b = win32gui.GetClientRect(self.hwnd)
        x, y = win32gui.ClientToScreen(self.hwnd, (l, t))
        self.regiao = {"left": x, "top": y, "width": r - l, "height": b - t}

    def capturar(self, sub=None):
        """Print da janela inteira, ou só de sub = (x, y, w, h) dentro dela (bem mais rápido)."""
        if self.hwnd and (sub is None or self.regiao is None):
            self._atualizar_regiao()
        if self._sct is None:
            self._sct = mss.mss()
        mon = self.regiao or self._sct.monitors[1]
        if sub:
            x, y, w, h = sub
            mon = {"left": mon["left"] + x, "top": mon["top"] + y, "width": w, "height": h}
        img = np.array(self._sct.grab(mon))
        return cv2.cvtColor(img, cv2.COLOR_BGRA2BGR)

    def com_foco(self):
        """True se o jogo está (ou foi trazido) para primeiro plano.

        Teclas e cliques vão para a janela em primeiro plano: sem esta checagem, o bot
        digitaria em outra janela (ex.: o chat do Claude) se ela estiver na frente.
        """
        if not self.hwnd:
            return True
        import win32gui
        if win32gui.GetForegroundWindow() == self.hwnd:
            return True
        trazer_para_frente(self.hwnd)
        if win32gui.GetForegroundWindow() == self.hwnd:
            return True
        print("Aviso: o jogo não está em primeiro plano; não vou apertar nada.", flush=True)
        return False

    def tecla(self, nome, segundos=0.05):
        """Aperta e solta uma tecla (nomes do pyautogui: "space", "left", "1"...)."""
        if not self.com_foco():
            return
        pyautogui.keyDown(nome)
        time.sleep(segundos)
        pyautogui.keyUp(nome)

    def segurar(self, nome):
        if self.com_foco():
            pyautogui.keyDown(nome)

    def soltar(self, nome):
        pyautogui.keyUp(nome)

    def clicar(self, x, y):
        if not self.com_foco():
            return
        ox = self.regiao["left"] if self.regiao else 0
        oy = self.regiao["top"] if self.regiao else 0
        # Alguns jogos ignoram clique sem o mouse "passar" antes por cima do botão.
        pyautogui.moveTo(ox + x, oy + y, duration=random.uniform(0.1, 0.3))
        time.sleep(0.05)
        pyautogui.click()

    def afastar_mouse(self):
        """Tira o mouse de cima do jogo (canto da janela) para sumir com balões de dica."""
        ox = self.regiao["left"] if self.regiao else 0
        oy = self.regiao["top"] if self.regiao else 0
        pyautogui.moveTo(ox + 5, oy + 5)

    def janela_valida(self):
        import win32gui
        return bool(self.hwnd and win32gui.IsWindow(self.hwnd))

    def preparar_verificacao(self):
        """Ativa somente por HWND; nunca clica no centro para recuperar foco."""
        import win32gui
        if not self.janela_valida():
            return False
        if win32gui.GetForegroundWindow() != self.hwnd:
            trazer_para_frente(self.hwnd)
        if win32gui.GetForegroundWindow() != self.hwnd:
            return False
        self._atualizar_regiao()
        return True

    def _verificacao_pronta(self, regiao_esperada):
        import win32gui
        if not self.janela_valida() or win32gui.GetForegroundWindow() != self.hwnd:
            return False
        self._atualizar_regiao()
        return self.regiao == regiao_esperada

    def capturar_verificacao(self):
        """Captura pixels novos em primeiro plano, inclusive no DriverConta."""
        if not self.preparar_verificacao():
            return None
        regiao = dict(self.regiao)
        frame = DriverTela.capturar(self)
        return (frame, regiao) if self._verificacao_pronta(regiao) else None

    def estacionar_mouse(self, x, y):
        """Põe o mouse em (x, y) da janela sem clicar (só com o jogo em primeiro plano).

        No captcha o mouse espera fora do teclado: com ele dentro da roda os botões somem.
        """
        import win32gui
        if not self.janela_valida() or win32gui.GetForegroundWindow() != self.hwnd:
            return False
        self._atualizar_regiao()
        if not (0 <= x < self.regiao["width"] and 0 <= y < self.regiao["height"]):
            return False
        pyautogui.moveTo(self.regiao["left"] + int(x), self.regiao["top"] + int(y), duration=0.05)
        return True

    def clicar_verificado(self, x, y, *, regiao_esperada, estacionar=None):
        """Aborta se foco, geometria ou janela sob o destino mudarem.

        `estacionar`: onde deixar o mouse depois do clique (padrão: canto da janela).
        """
        import win32gui
        if not self._verificacao_pronta(regiao_esperada):
            return False
        if not (0 <= x < self.regiao["width"] and 0 <= y < self.regiao["height"]):
            return False
        ponto = (self.regiao["left"] + int(x), self.regiao["top"] + int(y))

        def alvo_correto():
            return (self._verificacao_pronta(regiao_esperada)
                    and win32gui.GetAncestor(win32gui.WindowFromPoint(ponto), 2) == self.hwnd)

        if not alvo_correto():
            return False
        pyautogui.moveTo(*ponto, duration=0.08)
        if not alvo_correto():
            return False
        pyautogui.click(*ponto)
        # Tirar o mouse de cima do número antes da próxima leitura (o hover esconde os outros).
        fx, fy = estacionar if estacionar is not None else (5, 5)
        pyautogui.moveTo(self.regiao["left"] + int(fx), self.regiao["top"] + int(fy))
        return True

    @classmethod
    def da_janela(cls, titulo_parcial):
        """Cria o driver já preso à janela cujo título contém o texto."""
        ativar_dpi()
        hwnd = achar_hwnd(titulo_parcial)
        trazer_para_frente(hwnd)
        return cls(hwnd=hwnd)


def ativar_dpi():
    """Faz captura e clique usarem pixels reais mesmo com escala do Windows != 100%."""
    try:
        ctypes.windll.shcore.SetProcessDpiAwareness(2)
    except (AttributeError, OSError):
        pass


def trazer_para_frente(hwnd):
    import win32con
    import win32gui
    if win32gui.IsIconic(hwnd):
        win32gui.ShowWindow(hwnd, win32con.SW_RESTORE)
    if win32gui.GetForegroundWindow() == hwnd:
        return
    import win32api
    import win32process
    # O Windows não deixa um programa roubar a frente de outro. Três tentativas, da mais
    # leve à mais forte: (1) Alt + SetForegroundWindow; (2) "grudar" o bot na janela da
    # frente (AttachThreadInput) para herdar a permissão; (3) pôr a janela acima de todas
    # e de volta ao normal (o clique seguinte cai nela e o próprio clique dá o foco).
    for tentativa in range(3):
        try:
            if tentativa == 0:
                win32api.keybd_event(win32con.VK_MENU, 0, 0, 0)
                win32api.keybd_event(win32con.VK_MENU, 0, win32con.KEYEVENTF_KEYUP, 0)
                win32gui.SetForegroundWindow(hwnd)
            elif tentativa == 1:
                frente = win32gui.GetForegroundWindow()
                eu = win32api.GetCurrentThreadId()
                dela = win32process.GetWindowThreadProcessId(frente)[0] if frente else eu
                win32process.AttachThreadInput(eu, dela, True)
                try:
                    win32gui.BringWindowToTop(hwnd)
                    win32gui.SetForegroundWindow(hwnd)
                finally:
                    win32process.AttachThreadInput(eu, dela, False)
            else:
                flags = win32con.SWP_NOMOVE | win32con.SWP_NOSIZE
                win32gui.SetWindowPos(hwnd, win32con.HWND_TOPMOST, 0, 0, 0, 0, flags)
                win32gui.SetWindowPos(hwnd, win32con.HWND_NOTOPMOST, 0, 0, 0, 0, flags)
                win32gui.SetForegroundWindow(hwnd)
        except Exception:
            pass
        time.sleep(0.2)
        if win32gui.GetForegroundWindow() == hwnd:
            break
    time.sleep(0.3)


def listar_janelas():
    import win32gui
    janelas = []

    def cb(hwnd, _):
        titulo = win32gui.GetWindowText(hwnd)
        if win32gui.IsWindowVisible(hwnd) and titulo:
            janelas.append((hwnd, titulo))

    win32gui.EnumWindows(cb, None)
    return janelas


def achar_hwnd(titulo_parcial):
    achados = [h for h, t in listar_janelas() if titulo_parcial.lower() in t.lower()]
    if not achados:
        raise RuntimeError(f"Janela com '{titulo_parcial}' não encontrada.")
    return achados[0]
