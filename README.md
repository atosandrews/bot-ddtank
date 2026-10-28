# ddtank-bot

Projeto de estudo: automação do cliente DDTank Clássico com visão computacional.
Use apenas em servidor privado ou conta descartável (automação pode violar os termos de uso).

## Instalação (Windows)

```bash
python -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
```

## Passo a passo

1. Abra o jogo e rode `python tools/listar_janelas.py` para achar o título e o hwnd.
2. Tire prints dos botões (recortes pequenos) e salve em `img/` (ex.: `botao_iniciar.png`).
3. Teste um template: `python tools/testar_template.py img/botao_iniciar.png "TITULO DA JANELA"`
4. Rode o bot: `python -m src.main "TITULO DA JANELA"`

## Fases

- [x] Fase 1: captura da tela/janela + mouse real (`drivers/tela.py`)
- [ ] Fase 2: captura em segundo plano por hwnd (`drivers/janela.py`, experimental)
- [ ] Fase 3: várias instâncias (um processo por janela)
- [ ] Fase 4: (avançado) sem cliente

## Estrutura

- `src/drivers/`: como ver e agir na tela (trocável)
- `src/vision.py`: template matching
- `src/bot.py`: máquina de estados
- `src/main.py`: ponto de entrada
- `tools/`: scripts auxiliares
- `NOTAS.md`: contexto e progresso do projeto
- `img/`: templates; `logs/`: screenshots de debug
