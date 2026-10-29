# Catálogo de skills

Skills armazenadas em [`.agents/skills/`](../.agents/skills/). Cada pasta tem `SKILL.md` e `ORIGEM.txt` com o repositório de origem. Leia o `SKILL.md` indicado antes da etapa correspondente; as regras do [AGENTS.md](../AGENTS.md) têm precedência sobre qualquer skill.

## Como foram escolhidas

Busca feita em 2026-10-29 no índice que a documentação de [Agent Skills da Supabase](https://supabase.com/docs/guides/ai-tools/ai-skills) recomenda para descoberta (`npx skills find <consulta>`, diretório skills.sh). Node/npx não está instalado nesta máquina, então a busca usou direto a API `https://skills.sh/api/search` e os arquivos foram baixados do GitHub. Consultas: `python`, `pytest`, `opencv`, `ocr`, `onnx`, `computer vision`, `image processing`, `windows automation`, `python packaging`, `python testing`, `test-driven-development`, `systematic-debugging`, `verification-before-completion`, `code review`.

As skills oficiais da Supabase (`supabase/agent-skills`) **não** foram instaladas: nenhum dos dois projetos usa Supabase, banco ou HTTP entre si (ver [CONTEXTO](CONTEXTO.md)).

Cada `SKILL.md` foi lido antes da instalação. Em `systematic-debugging` foram removidos os arquivos de autoria/teste da própria skill e um script que depende de `npm`.

## Skills instaladas

| Skill | Origem | Dev | Tester | Uso neste projeto |
| --- | --- | :---: | :---: | --- |
| [systematic-debugging](../.agents/skills/systematic-debugging/SKILL.md) | obra/superpowers | ✔ | ✔ | Template não encontrado, tela errada, força/ângulo lidos errado, OCR sem candidato. Reproduzir com screenshot salvo antes de mexer em limiar |
| [test-driven-development](../.agents/skills/test-driven-development/SKILL.md) | obra/superpowers | ✔ | | Mudanças em `verificacao*.py`, `batalha.py` e funções puras: teste com driver/OCR/relógio simulado primeiro. Os exemplos da skill são TypeScript; aqui o comando é `pytest` |
| [verification-before-completion](../.agents/skills/verification-before-completion/SKILL.md) | obra/superpowers | ✔ | ✔ | Antes de dizer “passou”: rodar `pytest` e `tools/testar_prints.py` e citar saída e código de saída. Teste offline não é validação no jogo |
| [requesting-code-review](../.agents/skills/requesting-code-review/SKILL.md) | obra/superpowers | ✔ | | Antes de commit/merge |
| [python-testing-patterns](../.agents/skills/python-testing-patterns/SKILL.md) | wshobson/agents | | ✔ | Fixtures pytest, `Mock`/`monkeypatch` para drivers, relógio e `DdddOcr`; parametrizar sobre `docs/especificacao/imagens/` |
| [computer-vision-opencv](../.agents/skills/computer-vision-opencv/SKILL.md) | mindrally/skills | ✔ | ✔ | Template matching (`vision.py`), recortes, BGR/cinza, pré-processamento antes do OCR. Ignore a parte de PyTorch/GPU: o projeto usa CPU e ONNX |
| [python-packaging](../.agents/skills/python-packaging/SKILL.md) | wshobson/agents | ✔ | | `ddddocr/pyproject.toml`, instalação editável `-e ../ddddocr`, modelos `.onnx` como package-data. Não publicar no PyPI |

## Avaliadas e descartadas

- `smart-ocr`, `pdf-ocr-extraction`, `ocr-document-processor`, `silicon-paddle-ocr`: OCR de documentos/PDF ou outro motor; o projeto usa ddddocr.
- `windows-ui-automation`, `windows-desktop-e2e`, `anti-detect-browser`: automação genérica de UI/navegador; o bot tem drivers próprios e regras de foco próprias.
- `supabase`, `supabase-postgres-best-practices`: sem uso de Supabase.
- `python-executor`, `pytest-coverage` e variantes: redundantes com `python-testing-patterns`.

## Atualizar

Com Node instalado: `npx skills find <consulta>` e `npx skills add <repo> --skill <nome>`. Sem Node: baixe a pasta indicada em `ORIGEM.txt`, leia o `SKILL.md` e substitua a pasta. Registre a mudança neste catálogo.
