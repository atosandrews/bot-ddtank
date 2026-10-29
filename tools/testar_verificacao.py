"""Diagnóstico offline do perfil/OCR, sem importar drivers ou controlar o jogo."""
import argparse
import json
from pathlib import Path
import sys

import cv2

RAIZ = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(RAIZ))
from src.verificacao_ocr import ErroInfraestrutura, obter_ocr
from src.verificacao_visao import QuadroInvalido, VisaoVerificacao


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("imagem", type=Path)
    parser.add_argument("--sem-ocr", action="store_true", help="Validar apenas recortes/controles")
    parser.add_argument("--salvar", type=Path, help="Gravar recorte e marcações nesta pasta")
    args = parser.parse_args()
    frame = cv2.imread(str(args.imagem))
    if frame is None:
        parser.error(f"Não foi possível ler {args.imagem}")
    try:
        quadro = VisaoVerificacao().analisar(frame)
        if quadro is None:
            print("Quadro de verificação não encontrado")
            return 1
        resultado = {"origem": quadro.origem, "ocupados": quadro.ocupados,
                     "teclas": quadro.teclas, "confirmar": quadro.confirmar,
                     "resetar": quadro.resetar, "desafio": quadro.identidade}
        if not args.sem_ocr:
            from dataclasses import asdict
            from src import config
            ocr = obter_ocr()
            resultado["tempo"] = ocr.ler_tempo(quadro.tempo)
            resultado["candidatos"] = [asdict(c) for c in ocr.reconhecer(
                quadro.codigo, config.VERIFICACAO_LIMIAR_OCR)]
            resultado["leituras_ocr"] = ocr.ultimas_leituras
        print(json.dumps(resultado, ensure_ascii=False, indent=2))
        if args.salvar:
            args.salvar.mkdir(parents=True, exist_ok=True)
            cv2.imwrite(str(args.salvar / "codigo.png"), quadro.codigo)
            for texto, ponto in {**quadro.teclas, "OK": quadro.confirmar, "Reset": quadro.resetar}.items():
                cv2.circle(frame, ponto, 4, (0, 0, 255), 1)
                cv2.putText(frame, texto, ponto, cv2.FONT_HERSHEY_SIMPLEX, 0.4, (0, 255, 0), 1)
            cv2.imwrite(str(args.salvar / "controles.png"), frame)
        if not args.sem_ocr and not resultado["candidatos"]:
            print("Nenhum candidato elegível de quatro dígitos; não enviar ao jogo.")
            return 2
        return 0
    except (QuadroInvalido, ErroInfraestrutura) as exc:
        print(str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
