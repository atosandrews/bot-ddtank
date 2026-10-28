"""Fase 2 (experimental): captura e clique em segundo plano via hwnd.

Funciona com a janela coberta por outras, mas NÃO minimizada.
Alguns clientes ignoram PostMessage ou renderizam preto no PrintWindow;
nesse caso é preciso testar alternativas.
"""
import ctypes

import cv2
import numpy as np
import win32con
import win32gui
import win32ui


class DriverJanela:
    def __init__(self, hwnd):
        self.hwnd = hwnd

    def capturar(self):
        l, t, r, b = win32gui.GetClientRect(self.hwnd)
        w, h = r - l, b - t
        hwnd_dc = win32gui.GetWindowDC(self.hwnd)
        mfc_dc = win32ui.CreateDCFromHandle(hwnd_dc)
        save_dc = mfc_dc.CreateCompatibleDC()
        bmp = win32ui.CreateBitmap()
        bmp.CreateCompatibleBitmap(mfc_dc, w, h)
        save_dc.SelectObject(bmp)
        # 3 = PW_CLIENTONLY | PW_RENDERFULLCONTENT
        ctypes.windll.user32.PrintWindow(self.hwnd, save_dc.GetSafeHdc(), 3)
        info = bmp.GetInfo()
        buf = bmp.GetBitmapBits(True)
        img = np.frombuffer(buf, dtype=np.uint8).reshape(info["bmHeight"], info["bmWidth"], 4)
        win32gui.DeleteObject(bmp.GetHandle())
        save_dc.DeleteDC()
        mfc_dc.DeleteDC()
        win32gui.ReleaseDC(self.hwnd, hwnd_dc)
        return cv2.cvtColor(img, cv2.COLOR_BGRA2BGR)

    def clicar(self, x, y):
        lparam = (y << 16) | (x & 0xFFFF)
        win32gui.PostMessage(self.hwnd, win32con.WM_LBUTTONDOWN, win32con.MK_LBUTTON, lparam)
        win32gui.PostMessage(self.hwnd, win32con.WM_LBUTTONUP, 0, lparam)
