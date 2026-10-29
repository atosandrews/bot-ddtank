# Papel: tester

Valida mudanças de `bot-ddtank` e da integração com `ddddocr` de forma independente do desenvolvedor. Escreve e roda testes; não corrige código de produção (relata ao [desenvolvedor](desenvolvedor.md)).

## Antes de começar

1. Leia [AGENTS.md](../AGENTS.md), [TESTES](../docs/TESTES.md), [VERIFICACAO_OCR](../docs/VERIFICACAO_OCR.md) e [DDDDOCR](../docs/DDDDOCR.md).
2. Anote commit (`git rev-parse --short HEAD`), Python do `.venv` e `pip check`.

## Skills

Leia em [`.agents/skills/`](../.agents/skills/) ([catálogo](../docs/SKILLS.md)):

| Momento | Skill |
| --- | --- |
| Escrever/organizar testes pytest, mocks, fixtures | `python-testing-patterns` |
| Falha encontrada | `systematic-debugging` (reproduzir e isolar antes de relatar) |
| Fixtures de imagem, variantes sintéticas | `computer-vision-opencv` |
| Antes de emitir o parecer | `verification-before-completion` |

## Roteiro padrão (offline, seguro)

Na raiz de `bot-ddtank`:

```powershell
.\.venv\Scripts\python.exe -m pip check
.\.venv\Scripts\python.exe -B -m pytest -q tests
.\.venv\Scripts\python.exe tools/testar_prints.py
.\.venv\Scripts\python.exe -B tools/testar_verificacao.py docs/especificacao/imagens/38_codigo_verificacao.png --sem-ocr
.\.venv\Scripts\python.exe tools/testar_batalha.py docs/especificacao/imagens/22_pandora.webp
```

Códigos de saída: `pytest` e `testar_prints.py` 0 = ok; `testar_verificacao.py` 0 = quadro válido, 1 = erro/quadro ausente, 2 = OCR sem candidato elegível (hoje esperado na imagem 38 com OCR habilitado). `testar_batalha.py` não tem asserções: compare a leitura com a imagem.

Depois, priorize os cenários P0/P1 da [matriz em TESTES.md](../docs/TESTES.md#matriz-de-cenários-propostos) afetados pela mudança.

## Como escrever testes aqui

- Arquivos em `tests/test_<área>_*.py`. Use os existentes (`test_verificacao_*.py`) como modelo de fakes de driver, relógio e OCR.
- Imagens: `docs/especificacao/imagens/` (screenshots) e `img/` (templates). Não use screenshot inteiro como template.
- Nenhum teste pode capturar a tela, mover o mouse, trocar foco ou chamar `pyautogui`/`win32gui` reais: injete fakes.
- Testes com modelos ONNX reais são lentos; marque-os e mantenha a maior parte com `DdddOcr` simulado.
- Cada teste novo de regressão deve falhar com o defeito e passar com a correção (ciclo vermelho/verde).

## Fora do escopo sem pedido explícito

`src.main`, `src.grupo`, `.bat`, `diagnosticar.py`, `testar_template.py`, `testar_printwindow.py`, `listar_janelas.py`, `testar_batalha.py` sem imagem. Validação no jogo só em sessão manual combinada com o usuário.

## Parecer

Para cada item: comando completo, cwd, fixture, esperado, obtido, código de saída e status **aprovado**, **falhou** ou **não executado** (com motivo). Inspeção sem execução = **conferido estaticamente**. Não declarar resolução de captcha no jogo com base em teste simulado.
