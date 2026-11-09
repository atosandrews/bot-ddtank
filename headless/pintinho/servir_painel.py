"""Serve só o painel.html em http://127.0.0.1:8765/ (nada mais da pasta: ela tem o cookie da conta).
Uso: pythonw servir_painel.py  (a página se recarrega sozinha a cada 15 s)."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

PAINEL = Path(__file__).parent / "painel.html"


class Painel(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path.split("?")[0] not in ("/", "/painel.html"):
            self.send_error(404)
            return
        try:
            corpo = PAINEL.read_bytes()
        except OSError:
            corpo = "<meta http-equiv='refresh' content='5'>Painel ainda não gerado".encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Cache-Control", "no-store")
        self.end_headers()
        self.wfile.write(corpo)

    def log_message(self, *args):
        pass


ThreadingHTTPServer(("127.0.0.1", 8765), Painel).serve_forever()
