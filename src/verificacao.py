"""Controlador serial de leitura, preenchimento e confirmação da verificação."""
from dataclasses import dataclass
from datetime import datetime
from functools import lru_cache
import json
from pathlib import Path
import threading
import time

import cv2

from . import config, vision
from .telas import IMG
from .verificacao_ocr import ErroInfraestrutura, obter_ocr
from .verificacao_visao import QuadroInvalido, VisaoVerificacao

CODIGO = IMG / "codigo_verificacao.png"
_EXCLUSAO = threading.Lock()


@dataclass(frozen=True)
class Resultado:
    estado: str
    tentativas: int


@lru_cache(maxsize=1)
def obter_visao():
    return VisaoVerificacao()


def tem_codigo(frame):
    """Mantém o contrato usado pelos laços individual e de grupo."""
    return vision.achar(CODIGO, frame, config.VERIFICACAO_LIMIAR_TITULO) is not None


def preparar():
    """Carrega assets e modelos antes do laço, fora do prazo do captcha."""
    obter_visao()
    obter_ocr()


def _tela_conhecida(frame):
    from . import telas
    tela = telas.identificar(frame, config.LIMIAR)
    return tela is not None and tela.nome != "SEM_FOCO"


def _diagnostico(nome, quadro, candidato, tentativa):
    if not config.VERIFICACAO_SALVAR_DIAGNOSTICO:
        return
    pasta = Path(__file__).resolve().parent.parent / "logs" / "verificacao"
    pasta.mkdir(parents=True, exist_ok=True)
    identificador = datetime.now().strftime("%Y%m%d_%H%M%S_%f")
    cv2.imwrite(str(pasta / f"{identificador}.png"), quadro.codigo)
    (pasta / f"{identificador}.json").write_text(json.dumps({
        "conta": nome, "tentativa": tentativa, "candidato": candidato.texto,
        "confianca_heuristica": candidato.confianca, "modelo": candidato.modelo,
        "variante": candidato.variante, "desafio": quadro.identidade,
    }, ensure_ascii=False, indent=2), encoding="utf-8")


def resolver(driver, nome="bot", *, ocr=None, visao=None, dormir=time.sleep,
             relogio=time.monotonic, log=print, tela_conhecida=None):
    """Repete enquanto há desafio; Ctrl+C e FailSafeException não são interceptados.

    RESOLVIDO descreve confirmação seguida de retorno observado ao jogo, não uma
    resposta do servidor. EXPIRADO nunca é registrado como reconhecimento aceito.
    Dependências injetáveis permitem verificar toda a sequência sem mouse real.
    """
    ocr = ocr if ocr is not None else obter_ocr()
    visao = visao if visao is not None else obter_visao()
    tela_conhecida = tela_conhecida or _tela_conhecida
    for metodo in ("janela_valida", "preparar_verificacao", "capturar_verificacao", "clicar_verificado"):
        if not callable(getattr(driver, metodo, None)):
            raise ErroInfraestrutura(f"Driver sem suporte a verificação: {metodo}")

    def capturar():
        if not driver.janela_valida():
            raise ErroInfraestrutura(f"Janela de {nome} fechada ou indisponível")
        if not driver.preparar_verificacao():
            return None
        return driver.capturar_verificacao()

    def clicar_atual(base, destino, ocupados):
        if relogio() >= prazo:
            return False
        captura = capturar()
        if captura is None:
            return False
        frame, regiao = captura
        atual = visao.analisar(frame)
        if (atual is None or atual.identidade != base.identidade
                or atual.origem != base.origem or atual.teclas != base.teclas
                or atual.confirmar != base.confirmar or atual.resetar != base.resetar
                or atual.ocupados != ocupados):
            return False
        return driver.clicar_verificado(*destino, regiao_esperada=regiao)

    identidade = None
    rejeitados = set()
    tentativas = ausentes = 0
    inicio = relogio()
    prazo = inicio + config.VERIFICACAO_DURACAO_DESAFIO
    pendente = None
    expirado = False
    enviado_neste_desafio = False
    ultimo_tempo = None
    ultimo_aviso = float("-inf")
    log(f"[{nome}] verificação automática iniciada; Ctrl+C para interromper", flush=True)
    while True:
        captura = capturar()
        if captura is None:
            dormir(config.VERIFICACAO_INTERVALO)
            continue
        frame, _ = captura
        try:
            quadro = visao.analisar(frame)
            if quadro is None:
                if identidade is not None and relogio() >= prazo:
                    expirado = True
                ausentes = ausentes + 1 if tela_conhecida(frame) else 0
                if ausentes >= 2:
                    estado = "EXPIRADO" if expirado else ("RESOLVIDO" if enviado_neste_desafio else "RETOMADO")
                    log(f"[{nome}] verificação: {estado}; {tentativas} confirmação(ões)", flush=True)
                    return Resultado(estado, tentativas)
                dormir(config.VERIFICACAO_INTERVALO)
                continue
            ausentes = 0
            tempo = ocr.ler_tempo(quadro.tempo)
            novo_ciclo = (ultimo_tempo is not None and ultimo_tempo <= 3
                          and tempo is not None and tempo >= 20)
            if quadro.identidade != identidade or novo_ciclo:
                identidade = quadro.identidade
                rejeitados.clear()
                pendente = None
                inicio = relogio()
                prazo = inicio + config.VERIFICACAO_DURACAO_DESAFIO
                expirado = False
                enviado_neste_desafio = False
            if tempo is not None:
                ultimo_tempo = tempo
                prazo = min(prazo, relogio() + tempo)
            if relogio() >= prazo:
                expirado = True
            if expirado:
                if relogio() - ultimo_aviso >= 5:
                    log(f"[{nome}] prazo do desafio encerrado; aguardando nova imagem/estado", flush=True)
                    ultimo_aviso = relogio()
                dormir(config.VERIFICACAO_INTERVALO)
                continue
            if pendente is not None:
                texto, limite = pendente
                if relogio() < limite:
                    dormir(config.VERIFICACAO_INTERVALO)
                    continue
                rejeitados.add(texto)
                log(f"[{nome}] tentativa {tentativas} sem aceitação observada; reavaliando", flush=True)
                pendente = None
            if quadro.ocupados:
                # Resetar pode trocar o código/teclado: sempre reanalisar no próximo ciclo.
                if clicar_atual(quadro, quadro.resetar, quadro.ocupados):
                    dormir(config.VERIFICACAO_ESPERA_CLIQUE)
                else:
                    dormir(config.VERIFICACAO_INTERVALO)
                continue
            candidatos = ocr.reconhecer(quadro.codigo, config.VERIFICACAO_LIMIAR_OCR)
            candidato = next((c for c in candidatos if c.texto not in rejeitados), None)
            if candidato is None:
                if relogio() - ultimo_aviso >= 5:
                    log(f"[{nome}] sem candidato novo de quatro dígitos; recapturando", flush=True)
                    ultimo_aviso = relogio()
                dormir(config.VERIFICACAO_INTERVALO)
                continue
            inicio_tentativa = relogio()
            for indice, digito in enumerate(candidato.texto):
                if not clicar_atual(quadro, quadro.teclas[digito], indice):
                    break
                dormir(config.VERIFICACAO_ESPERA_CLIQUE)
            else:
                # Não confirmar se o tempo se esgotou durante OCR/preenchimento.
                if relogio() < prazo:
                    captura_final = capturar()
                    if captura_final is not None:
                        final = visao.analisar(captura_final[0])
                        if final is not None and ocr.ler_tempo(final.tempo) != 0:
                            if clicar_atual(quadro, quadro.confirmar, 4):
                                tentativas += 1
                                enviado_neste_desafio = True
                                pendente = (candidato.texto, relogio()+config.VERIFICACAO_ESPERA_RESPOSTA)
                                _diagnostico(nome, quadro, candidato, tentativas)
                                log(f"[{nome}] tentativa {tentativas}: {candidato.texto} "
                                    f"({candidato.modelo}/{candidato.variante}, "
                                    f"score={candidato.confianca:.3f}, "
                                    f"duracao={relogio()-inicio_tentativa:.2f}s)", flush=True)
            dormir(config.VERIFICACAO_INTERVALO)
        except QuadroInvalido as exc:
            ausentes = 0
            if relogio() - ultimo_aviso >= 5:
                log(f"[{nome}] sem clique: {exc}", flush=True)
                ultimo_aviso = relogio()
            dormir(config.VERIFICACAO_INTERVALO)


def tratar(driver, nome="bot"):
    with _EXCLUSAO:
        resultado = resolver(driver, nome)
    if resultado.estado == "EXPIRADO":
        raise ErroInfraestrutura("Verificação expirou sem aceitação observada; fluxo interrompido")
    return resultado
