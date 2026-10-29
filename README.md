# ddtank-bot

Projeto de estudo: automação do cliente DDTank Clássico com visão computacional.
Use apenas em servidor privado ou conta descartável (automação pode violar os termos de uso).

## Como rodar (Windows)

1. Instale o Python 3 (marque "Add python.exe to PATH").
2. Abra o jogo na tela de seleção de canal.
3. Dê duplo clique em `rodar_bot.bat`. Na primeira vez ele cria o `.venv` e instala as dependências.
4. Escolha a janela do jogo na lista (ou salve o título em `src/config.py`).

O bot segue o fluxo de `docs/especificacao/ESPECIFICACAO.md`:
seleção de canal → (popup "aluno recomendado") → lobby → Central → Cais de expedição → Jogar.
Para parar: `Ctrl+C` no terminal, ou jogue o mouse num canto da tela.

## Ferramentas

- `python tools/testar_verificacao.py docs/especificacao/imagens/38_codigo_verificacao.png`: diagnóstico offline do OCR e dos controles, sem clicar. Veja [verificação automática](docs/VERIFICACAO_OCR.md) para instalação, testes e limitações de reconhecimento.
- `python tools/testar_prints.py`: testa o reconhecimento nos prints da especificação, sem o jogo.
- `python tools/testar_batalha.py [print]`: mostra o que o bot lê da batalha e a força que usaria, sem apertar nada.
- `python tools/diagnosticar.py`: tira um print do jogo e mostra qual tela o bot reconhece, sem clicar.
- `python tools/listar_janelas.py`: lista os títulos das janelas abertas.
- `python tools/testar_template.py img/canal_2.png "TITULO DA JANELA"`: testa um template no jogo aberto.

## Adicionar uma tela nova

1. Recorte um pedaço que identifica a tela e o botão a clicar; salve em `img/`.
2. Adicione uma linha em `TELAS` (`src/telas.py`).
3. Coloque o print da tela em `docs/especificacao/imagens/` e em `tools/testar_prints.py`.

## Fases

- [x] Fase 1: captura da tela/janela + mouse real (`drivers/tela.py`)
- [ ] Fase 2: captura em segundo plano por hwnd (`drivers/janela.py`, experimental)
- [ ] Fase 3: várias instâncias (um processo por janela)
- [ ] Fase 4: (avançado) sem cliente

## Estrutura

- `src/drivers/`: como ver e agir na tela (trocável)
- `src/vision.py`: template matching
- `src/telas.py`: telas conhecidas e a ação de cada uma
- `src/bot.py`: laço principal (reconhece a tela, clica, confere se mudou)
- `src/config.py`: título da janela, limiar e esperas
- `src/main.py`: ponto de entrada
- `docs/especificacao/`: especificação do fluxo e prints do jogo
- `tools/`: scripts auxiliares
- `NOTAS.md`: contexto e progresso do projeto
- `img/`: templates; `logs/`: screenshots de debug


## Documentação local em português

- [Verificação automática com OCR: implementação e resultados](docs/VERIFICACAO_OCR.md)
- [Instruções para agentes](AGENTS.md)
- [Contexto atual e arquitetura](docs/CONTEXTO.md)
- [Ambiente e desenvolvimento](docs/DESENVOLVIMENTO.md)
- [Testes e evidências](docs/TESTES.md)
- [Catálogo compartilhado de skills](../docs/SKILLS.md)
