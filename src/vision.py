import cv2

_cache = {}


def carregar(path):
    if path not in _cache:
        tpl = cv2.imread(path)
        if tpl is None:
            raise FileNotFoundError(f"Template não encontrado: {path}")
        _cache[path] = tpl
    return _cache[path]


def achar(template_path, frame, limiar=0.85):
    """Retorna (x, y) do centro do template no frame, ou None."""
    tpl = carregar(template_path)
    res = cv2.matchTemplate(frame, tpl, cv2.TM_CCOEFF_NORMED)
    _, val, _, loc = cv2.minMaxLoc(res)
    if val >= limiar:
        h, w = tpl.shape[:2]
        return loc[0] + w // 2, loc[1] + h // 2
    return None
