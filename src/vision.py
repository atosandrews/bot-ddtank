import cv2

_cache = {}


def carregar(path):
    path = str(path)
    if path not in _cache:
        tpl = cv2.imread(path)
        if tpl is None:
            raise FileNotFoundError(f"Template não encontrado: {path}")
        _cache[path] = tpl
    return _cache[path]


def procurar(template_path, frame):
    """Retorna ((x, y), confianca) do melhor ponto do template no frame.

    (x, y) é o centro do template; confianca vai de -1 a 1 (1 = idêntico).
    """
    tpl = carregar(template_path)
    h, w = tpl.shape[:2]
    if h > frame.shape[0] or w > frame.shape[1]:
        return None, 0.0
    res = cv2.matchTemplate(frame, tpl, cv2.TM_CCOEFF_NORMED)
    _, val, _, loc = cv2.minMaxLoc(res)
    return (loc[0] + w // 2, loc[1] + h // 2), val


def achar(template_path, frame, limiar=0.85):
    """Retorna (x, y) do centro do template no frame, ou None."""
    pos, val = procurar(template_path, frame)
    return pos if val >= limiar else None
