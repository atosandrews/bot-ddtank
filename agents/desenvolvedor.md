# Papel: desenvolvedor

Altera `bot-ddtank` e, quando necessário, a integração com o SDK irmão `ddddocr`. Entrega mudança pequena, testada offline e documentada.

## Antes de começar

1. Leia [AGENTS.md](../AGENTS.md), [CONTEXTO](../docs/CONTEXTO.md), [DESENVOLVIMENTO](../docs/DESENVOLVIMENTO.md) e, se tocar em captcha/OCR, [VERIFICACAO_OCR](../docs/VERIFICACAO_OCR.md) e [DDDDOCR](../docs/DDDDOCR.md).
2. Confira o ambiente: `.\.venv\Scripts\python.exe --version` (esperado 3.13.x). Não use o `py` global (3.15).
3. Rode a linha de base e guarde o resultado:

```powershell
.\.venv\Scripts\python.exe -B -m pytest -q tests
.\.venv\Scripts\python.exe tools/testar_prints.py
```

## Skills

Leia em [`.agents/skills/`](../.agents/skills/) ([catálogo](../docs/SKILLS.md)):

| Momento | Skill |
| --- | --- |
| Bug ou comportamento inesperado | `systematic-debugging` |
| Implementar ou corrigir | `test-driven-development` |
| Visão, recortes, pré-processamento | `computer-vision-opencv` |
| `pyproject.toml`/instalação do ddddocr | `python-packaging` |
| Antes de afirmar que terminou | `verification-before-completion` |
| Antes de commit/merge | `requesting-code-review` |

## Mapa de onde mexer

| Pedido | Arquivos | Teste mínimo |
| --- | --- | --- |
| Tela nova ou reconhecimento | `img/*.png`, `src/telas.py` (`TELAS`, ordem = prioridade), `docs/especificacao/imagens/` | Nova linha em `ESPERADO` de `tools/testar_prints.py`; todas as expectativas passam |
| Template matching | `src/vision.py`, limiares em `src/config.py` | `testar_prints.py`; não reduzir limiar só para um positivo passar |
| Batalha (estágio, vez, ângulo, força) | `src/batalha.py`, estratégias em `src/config.py` | `tools/testar_batalha.py <imagem>` + teste pytest de função pura |
| Grupo (convites, Pronto, foco) | `src/grupo.py`, `src/drivers/conta.py` | Driver simulado; ao vivo só com pedido do usuário |
| Captcha/verificação | `src/verificacao.py`, `verificacao_visao.py`, `verificacao_ocr.py` | `tests/test_verificacao_*.py`; `tools/testar_verificacao.py <img> --sem-ocr` |
| Captura/ação | `src/drivers/tela.py`, `janela.py`, `conta.py` | Substituir driver por fake; preservar foco e liberação de teclas |
| SDK OCR | `../ddddocr/ddddocr/` (repositório separado) | Smoke do adaptador; ver [DDDDOCR](../docs/DDDDOCR.md). Preferir ajustar o adaptador do bot a alterar o SDK |

## Regras

- Não execute `src.main`, `src.grupo`, `.bat` nem ferramentas de captura sem pedido explícito: mexem mouse/teclado e janelas reais.
- Visão recebe imagem; captura/ação ficam nos drivers. Código novo deve aceitar imagem salva e driver/relógio injetáveis.
- Templates sem hover; registre resolução, escala/DPI e região do cliente.
- Não usar HTTP/FastAPI/MCP do ddddocr; o bot importa `DdddOcr` direto.
- Atualize `docs/` quando o comportamento mudar; acrescente evidências datadas em `NOTAS.md` sem reescrever histórico.

## Entrega

Arquivos alterados, motivo, comandos com código de saída e resultado observado. Separe: **conferido estaticamente**, **teste offline** e **execução no jogo** (esta só quando realmente feita). Peça ao [tester](tester.md) a validação independente quando a mudança afeta reconhecimento, batalha ou verificação.
