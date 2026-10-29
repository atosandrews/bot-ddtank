# Projeto irmão ddddocr — guia para desenvolvimento e testes

Inspeção em 2026-10-29 do checkout `ddtank-context/ddddocr` (commit `c40f56f`, versão 1.6.1, sem alterações locais). É um repositório Git independente, cópia do projeto público [sml2h3/ddddocr](https://github.com/sml2h3/ddddocr). O bot o usa como **SDK local**, instalado em modo editável pelo `requirements.txt` (`-e ../ddddocr`).

## Layout esperado

```
ddtank-context/
├── bot-ddtank/   ← este repositório (.venv próprio, Python 3.13)
└── ddddocr/      ← SDK; precisa estar lado a lado para o -e ../ddddocr
```

Dentro de um worktree do bot (por exemplo `.claude/worktrees/...`), `../ddddocr` não existe: rode instalação e testes no checkout `bot-ddtank` ou ajuste o caminho só no ambiente local.

## O que o bot usa

| Parte do SDK | Uso no bot |
| --- | --- |
| `ddddocr.DdddOcr` (`ddddocr/compat/v1.py`) | Duas instâncias CPU em `src/verificacao_ocr.py`: `beta=False` (`common_old.onnx`) e `beta=True` (`common.onnx`), com `show_ad=False`, `use_gpu=False` |
| `set_ranges("0123456789")` | Restringe a saída a dígitos |
| `classification(png_bytes, probability=True)` | O adaptador lê `text` e `confidence` e aceita só 4 dígitos com confiança ≥ limiar |
| `ddddocr/*.onnx` | Modelos (~88 MB no total) carregados como package-data; não versionar cópias no bot |

Não usados e fora do escopo: `ddddocr/api/` (FastAPI, servidor HTTP, `mcp.py`), `detection`, `slide_match`/`slide_comparison`, GPU, Docker. O bot não faz chamadas HTTP.

## Estrutura do SDK

| Pasta | Conteúdo |
| --- | --- |
| `ddddocr/compat/v1.py` | Classe pública `DdddOcr` (fachada compatível com a API antiga) |
| `ddddocr/core/` | `ocr_engine.py` (pré-processamento, inferência ONNX, decodificação CTC, probabilidades), `detection_engine.py`, `slide_engine.py` |
| `ddddocr/models/` | Carregamento de modelos e charsets |
| `ddddocr/preprocessing/` | Filtro de cor e processamento de imagem |
| `ddddocr/utils/` | Validação de entrada, exceções (`DdddOcrInputError`, `InvalidImageError`), I/O de imagem |
| `examples/` | Exemplos de uso; `api_client.py` é HTTP e não se aplica |

O README do SDK é em chinês; as seções úteis são “初始化参数详解” (parâmetros), “OCR概率输出” (probabilidades) e “自定义字符范围” (charset).

## Desenvolvimento

- Prefira mudar o adaptador `src/verificacao_ocr.py` (variantes de imagem, limiar, filtragem) a alterar o SDK. Ele isola o bot das interfaces do ddddocr.
- Se precisar mudar o SDK: branch própria no repositório `ddddocr`, sem alterar assinaturas públicas de `DdddOcr`, e registre no [VERIFICACAO_OCR](VERIFICACAO_OCR.md) qual commit do SDK o bot exige.
- Dependências do SDK (`pyproject.toml`): numpy, onnxruntime, Pillow, opencv-python. Os extras `api` não são instalados pelo bot.
- Skill útil: [python-packaging](../.agents/skills/python-packaging/SKILL.md) para `pyproject.toml`/instalação editável.

## Testes

O ddddocr **não tem suíte de testes** no checkout. A cobertura da integração fica no bot:

| Camada | Onde | Observação |
| --- | --- | --- |
| Contrato do adaptador com OCR simulado | `tests/test_verificacao_ocr.py` | Injeta `fabrica` falsa em `OcrLocal`; rápido, sem ONNX |
| Fluxo/visão/driver/integração | `tests/test_verificacao_*.py` | OCR, relógio e drivers simulados |
| Smoke com modelos reais | `tools/testar_verificacao.py <img>` (sem `--sem-ocr`) | Carrega ONNX; hoje a imagem 38 retorna saída 2 (sem candidato). Isso é limitação conhecida, não regressão |
| Instalação | `.\.venv\Scripts\python.exe -c "import ddddocr; print(ddddocr.__file__)"` | Deve apontar para `..\ddddocr\ddddocr\__init__.py` |

Para medir acurácia, faltam imagens de captcha rotuladas (ver pendências em [VERIFICACAO_OCR](VERIFICACAO_OCR.md#validação-realizada-e-pendências)). Guarde-as com o código correto no nome e compare candidatos por modelo/variante; acerto em imagem sintética não comprova acerto no jogo.
