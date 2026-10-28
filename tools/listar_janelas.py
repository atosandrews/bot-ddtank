import win32gui


def cb(hwnd, _):
    if win32gui.IsWindowVisible(hwnd):
        titulo = win32gui.GetWindowText(hwnd)
        if titulo:
            print(hwnd, "|", titulo)


win32gui.EnumWindows(cb, None)
