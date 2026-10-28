"""Fase 1: captura a tela (ou uma região) e usa o mouse real."""
import ctypes
import random
import time

import cv2
import mss
import numpy as np
import pyautogui


class DriverTela:
    def __init__(self, regiao=None, hwnd=None):
        # regiao = {"left": x, "top": y, "width": w, "height": h}
        # Com hwnd, a região acompanha a área interna da janela a cada captura.
        self.regiao = regiao
        self.hwnd = hwnd

    def _atualizar_regiao(self):
        import win32gui
        l, t, r, b = win32gui.GetClientRect(self.hwnd)
        x, y = win32gui.ClientToScreen(self.hwnd, (l, t))
        self.regiao = {"left": x, "top": y, "width": r - l, "height": b - t}

    def capturar(self):
        if self.hwnd:
            self._atualizar_regiao()
        with mss.mss() as sct:
            mon = self.regiao or sct.monitors[1]
            img = np.array(sct.grab(mon))
            return cv2.cvtColor(img, cv2.COLOR_BGRA2BGR)

    def clicar(self, x, y):
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
    try:
        win32gui.SetForegroundWindow(hwnd)
    except Exception:
        print("Aviso: não consegui trazer a janela para frente; clique nela uma vez.")
    time.sleep(0.5)


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
