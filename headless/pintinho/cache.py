"""Extrai entradas do cache blockfile do Chromium (somente leitura)."""
import os, struct, sys, zlib, gzip, re
CACHE = r"C:\Users\ATOS I\AppData\Roaming\DDClassico\Cache"
OUT = sys.argv[1]
pat = re.compile(sys.argv[2]) if len(sys.argv) > 2 else None
files = {}
def blockfile(n):
    if n not in files:
        files[n] = open(os.path.join(CACHE, f"data_{n}"), "rb").read()
    return files[n]
SIZES = {2: 256, 3: 1024, 4: 4096}
def read(addr, size):
    if not addr & 0x80000000: return None
    t = (addr >> 28) & 7
    if t == 0:
        p = os.path.join(CACHE, "f_%06x" % (addr & 0x0FFFFFFF))
        return open(p, "rb").read()[:size] if os.path.exists(p) else None
    bs = SIZES.get(t)
    if not bs: return None
    f = (addr >> 16) & 0xFF; start = addr & 0xFFFF
    d = blockfile(f)
    off = 8192 + start * bs
    return d[off:off + size]
d1 = blockfile(1)
n = 0
for off in range(8192, len(d1) - 256, 256):
    e = d1[off:off + 256]
    key_len, long_key = struct.unpack_from("<iI", e, 32)
    if key_len <= 0 or key_len > 4096: continue
    sizes = struct.unpack_from("<4i", e, 40); addrs = struct.unpack_from("<4I", e, 56)
    if key_len <= 160 - 1 and not long_key:
        key = e[96:96 + key_len]
    else:
        k = read(long_key, key_len); key = k or b""
    try: key = key.decode("latin1")
    except: continue
    if not key.startswith(("http", "1/0/_dk")) and "http" not in key: continue
    url = key[key.find("http"):]
    if pat and not pat.search(url): continue
    body = read(addrs[1], sizes[1]) if sizes[1] > 0 else None
    if not body: continue
    name = "%05d.bin" % n
    open(os.path.join(OUT, name), "wb").write(body)
    n += 1
    print(name, sizes[1], url)
print("total", n)
