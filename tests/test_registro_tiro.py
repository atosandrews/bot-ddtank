from pathlib import Path

import cv2
import pytest

from src import batalha, registro
from src.batalha import Batalha, ler

RAIZ = Path(__file__).resolve().parents[1]


class Driver:
    def __init__(self, frame):
        self.frame = frame

    def capturar(self):
        return self.frame


@pytest.fixture
def frame():
    imagem = cv2.imread(str(RAIZ / "docs/especificacao/imagens/24_formigas_minha_vez_janela_real.png"))
    assert imagem is not None
    return imagem


def test_tiro_registra_posicoes_lado_e_agulha_e_salva_print_anotado(frame, tmp_path):
    leitura = ler(frame)
    assert leitura is not None and leitura.eu is not None and leitura.inimigos
    linhas = []
    b = Batalha(Driver(frame), linhas.append, lambda *a: None)
    b.turno, b.alvo_dx = 3, int(leitura.inimigos[0][0] - leitura.eu[0])
    b.registrar_tiro(frame, leitura, 1, batalha.lado_virado(frame, leitura.ancora), virar=True)
    assert len(linhas) == 1
    for trecho in ("tiro turno 3", "formigas dx=", f"alvo dx={b.alvo_dx}", "lado=direita",
                   "agulha=", "seta=sim"):
        assert trecho in linhas[0]
    arquivos = list((tmp_path / "tiros").glob("*_t3_direita.jpg"))
    assert len(arquivos) == 1
    salvo = cv2.imread(str(arquivos[0]))
    assert salvo.shape == frame.shape


def test_sem_print_quando_desligado(frame, tmp_path, monkeypatch):
    monkeypatch.setattr(batalha.config, "SALVAR_PRINT_TIRO", False)
    leitura = ler(frame)
    linhas = []
    Batalha(Driver(frame), linhas.append, lambda *a: None).registrar_tiro(
        frame, leitura, -1, None, virar=False)
    assert "lado=esquerda" in linhas[0] and "agulha=indefinido" in linhas[0]
    assert not (tmp_path / "tiros").exists()


def test_log_do_terminal_vai_para_arquivo_do_dia(tmp_path):
    registro.gravar_log("linha um")
    registro.imprimir("parado:", "teste")
    arquivos = list(tmp_path.glob("execucao_*.txt"))
    assert len(arquivos) == 1
    texto = arquivos[0].read_text(encoding="utf-8")
    assert "linha um" in texto and "parado: teste" in texto
