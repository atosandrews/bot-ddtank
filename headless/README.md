# Formigueiro headless (sem cliente, sem tela)

Grupo de 4 personagens farmando o Formigueiro / Normal em loop, sem o cliente do jogo: o bot em Go
(`codigo/bot`) decide e mira, o relay (`codigo/ddrelay`) fala com o servidor, e tudo roda num
contêiner Docker. Diferente do bot Python deste repositório, que vê a tela e usa mouse/teclado.

Esta pasta é uma cópia versionada da instalação em uso, `C:\Users\ATOS I\ddbot\` (os scripts
apontam para esse caminho). Origem do código: zips `bot`, `ddrelay`, `proto` e `contract`;
o que foi criado ou alterado está em [codigo/LEIA-ME.md](codigo/LEIA-ME.md).

## Peças

| Arquivo | Papel |
| --- | --- |
| `iniciar.ps1` / `parar.ps1` | sobe/para o contêiner `ddbot-grupo` (`--restart unless-stopped`) e abre o painel |
| `parar_e_relatorio.ps1` | para o grupo e grava `relatorio-<data>.txt` |
| `codigo/run/bot-all.sh` | dentro do contêiner: compila, sobe bot + relay + login, vigia e renova o cookie |
| `codigo/run/renovar_cookie.py` | o site devolve um `.Auth` novo (14 dias) a cada visita; renovado a cada 6 h |
| `formigueiro-grupo.lua` | script de cada personagem: Formigueiro em grupo, recuperação, mochila, captcha, carteira |
| `ocr_worker.py` | captcha pelo OCR deste repositório (`src/verificacao_ocr.py` + SDK `../ddddocr`) |
| `painel.py --ao-vivo` | gera `painel.html` a cada 15 s, soma o farm em `farm.json` e mantém o PC acordado |
| `bot.yaml`, `relay.yaml`, `headless*.yaml` | configs do bot, do relay e dos 4 personagens (IDs, servidor S1) |

## Montar numa máquina nova

1. Copie esta pasta para `C:\Users\ATOS I\ddbot\` e tenha `ddtank-context\ddddocr` e `ddtank-context\bot-ddtank` ao lado.
2. Salve o cookie `.Auth` da conta em `C:\Users\ATOS I\ddbot\sessao.json` como `{ "cookie": "..." }` (nunca versionar).
3. Docker Desktop com virtualização ligada na BIOS e WSL instalado; rode `iniciar.ps1`.

## Fora do Git (`.gitignore`)

Cookie (`sessao*.json`), contagem e relatórios (`farm.json`, `relatorio-*.txt`, `painel.html`),
captchas salvos, `codigo/run/lua.tar` e o modelo `codigo/bot/models/captcha/model.onnx` (54 MB,
só usado se `captcha.worker` for removido do `bot.yaml`).
