"""Converte dead.png (alpha > 150 = sólido) no módulo Lua data/terrain/<id>.lua.
Uso: terreno.py <png> <map_id> <saida.lua> [altura]; altura corta as linhas abaixo da altura do mapa
(o dead.png do 1162 tem 1106 linhas e o mapa 1049; as linhas cortadas ficam fora do mapa)."""
import gzip, hashlib, sys
from pathlib import Path
from PIL import Image
src, map_id, out = sys.argv[1], sys.argv[2], sys.argv[3]
data = Path(src).read_bytes()
if data[:2] == b"\x1f\x8b": data = gzip.decompress(data)
print(map_id, "sha256", hashlib.sha256(data).hexdigest())
img = Image.open(__import__("io").BytesIO(data)).convert("RGBA")
w, h = img.size
if len(sys.argv) > 4: h = min(h, int(sys.argv[4]))
a = img.getchannel("A").load()
lines = [f"return {{width={w},height={h},rows={{"]
for y in range(h):
    spans, x = [], 0
    while x < w:
        if a[x, y] > 150:
            s = x
            while x < w and a[x, y] > 150: x += 1
            spans += [s, x]
        else:
            x += 1
    if spans: lines.append(f"[{y}]={{" + ",".join(map(str, spans)) + "},")
lines.append("}}")
Path(out).write_text("\n".join(lines) + "\n", encoding="utf-8")
print(w, h)
