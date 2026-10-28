"""Fase 1: captura a tela (ou uma região) e usa o mouse real."""
import cv2
import mss
import numpy as np
import pyautogui


class DriverTela:
    def __init__(self, regiao=None):
        # regiao = {"left": x, "top": y, "width": w, "height": h}
        self.regiao = regiao

    def capturar(self):
        with mss.mss() as sct:
            mon = self.regiao or sct.monitors[1]
            img = np.array(sct.grab(mon))
            return cv2.cvtColor(img, cv2.COLOR_BGRA2BGR)

    def clicar(self, x, y):
        ox = self.regiao["left"] if self.regiao else 0
        oy = self.regiao["top"] if self.regiao else 0
        pyautogui.click(ox + x, oy + y)

    @classmethod
    def da_janela(cls, titulo_parcial):
        """Cria o driver já com a região da janela cujo título contém o texto."""
        import win32gui
        hwnd = achar_hwnd(titulo_parcial)
        l, t, r, b = win32gui.GetWindowRect(hwnd)
        return cls({"left": l, "top": t, "width": r - l, "height": b - t})


def achar_hwnd(titulo_parcial):
    import win32gui
    achados = []

    def cb(hwnd, _):
        if win32gui.IsWindowVisible(hwnd) and titulo_parcial.lower() in win32gui.GetWindowText(hwnd).lower():
            achados.append(hwnd)

    win32gui.EnumWindows(cb, None)
    if not achados:
        raise RuntimeError(f"Janela com '{titulo_parcial}' não encontrada.")
    return achados[0]
