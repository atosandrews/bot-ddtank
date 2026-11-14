# Código do bot headless (bot + ddrelay + api)

Montado a partir de `Downloads\bot`, `Downloads\ddrelay`, `Downloads\proto` e `Downloads\contract`
em 31/10/2026. Compila e roda no Docker (`Dockerfile` nesta pasta, imagem `ddbot-build`).

## Criado aqui (não vinha nos zips)

- `api/go.mod`, `go.work` — módulo `ddbot/api` (proto + contract) e workspace dos três módulos.
- `run/bot-all.sh` — sobe bot, relay e login dos personagens dentro do contêiner.

## Alterado em relação aos originais

| Arquivo | Motivo |
| --- | --- |
| `bot/lua/rewards.lua`, `bot/lua/inventory.lua`, `bot/lua/instances/party.lua`, `bot/lua/instances/formigueiro.lua` | gopher-lua cria um vetor até a chave inteira: tabelas indexadas por template_id/player_id passaram a usar `tostring` (11 GB → ~300 MB, CPU ~600% → ~10%) |
| `bot/lua/inventory.lua` | pausa por falta de confirmação só vale para comando realmente enviado (`sent`), após 60 s |
| `bot/lua/instances/formigueiro.lua` | quadro de cartas fechado com carta faltando conta como partida concluída (registra `cards_shortfall`); status expõe `preflight` |
| `bot/lua/rewards.lua`, `bot/lua/instances/formigueiro.lua` | mochila cheia não para o loop: arrumação que falha é pulada, recompensa que não coube fecha a partida como `undelivered` (contada em `undelivered_runs`) |
| `bot/lua/rewards.lua` | arrumar a mochila só depois que uma recompensa não coube (antes: a cada coleta, segurando o grupo) |
| `bot/lua/rewards.lua`, `bot/lua/instances/formigueiro.lua` | sempre move a bolsa temporária (pacote 108); se não esvaziar em 6 s, arruma a mochila e tenta de novo; se a temporária esvazia sem a mochila ganhar (5 s), conta como enviado ao Correio (`mailed_runs`) e arruma na coleta seguinte |
| `ddrelay/internal/headless/peer.go` | envia um heartbeat logo após o login, para o relay aprender a chave e aceitar comandos desde o início (captcha logo ao entrar falhava com `packet_boundary_unavailable`) |
| `bot/lua/instances/formigueiro.lua` | resposta de captcha não escrita (`stale_state` ou `packet_boundary_unavailable`) é reenviada até 30 vezes |
| `bot/lua/instances/formigueiro.lua` | carta extra do quadro final que não chega em 3 s (personagem sem direito) encerra a escolha e deixa de pedir extra por 10 partidas |
| `run/renovar_cookie.py`, `run/bot-all.sh` | o site devolve um `.Auth` novo (14 dias) a cada visita: renovado após o login e a cada 6 h, guardado em `run/sessao_atual.json` (usado no próximo início se for mais novo que `ddbot\sessao.json`) |
| `bot/lua/instances/formigueiro.lua` | carta de ouro (template -100, "Moedas de ouro") é registrada na entrega do quadro como `received["-100"]`; antes era ignorada por não ir para a mochila |
| `ddrelay/internal/app/headless_runtime.go` | personagem que perde a conexão refaz o login sozinho (5 s, dobrando até 1 min; desiste após 10 min falhando), sem derrubar os outros |
| `run/bot-all.sh` | vigia também reinicia o grupo se faltar personagem conectado por 2 checagens seguidas (~2 min) |
| `ddbot\formigueiro-grupo.lua` (fora desta pasta) | Formigueiro recriado só conta partidas a partir da primeira luta que vê começar (antes contava a última luta de novo a cada recriação) |
| `run/bot-all.sh` | vigia: sai (e o Docker reinicia com `--restart unless-stopped`) se bot/relay/login morrerem ou se nenhuma partida terminar em 15 min |
| `run/bot-all.sh` | Lua empacotado em `run/lua.tar` e extraído em `/tmp/lua` (ler 13 mil arquivos pela pasta do Windows levava 25–50 s) |
| `bot/internal/captcha/worker.go`, `bot/internal/app/captcha*.go` | opção `captcha.worker`: reconhecimento por processo externo (OCR do bot-ddtank) |
| `ddrelay/internal/headless/authenticator.go` | erro de login mostra o ID devolvido pelo servidor |
| `ddrelay/internal/link/connection.go`, `run.go` | motivo de desconexão e de falha do script aparece no status da sessão |

`bot/internal/captcha/amostras*_debug_test.go` são diagnósticos com captchas reais; pulam sem as variáveis `AMOSTRAS`/`OCR_WORKER`.

## Login e recuperação — 13/11/2026

O relay passou a oferecer transporte candidato `launcher_chromium87`, diagnóstico `-check-auth`, autenticação/reconexão por personagem, releitura do cookie e circuito compartilhado contra bloqueios HTTP. Os supervisores locais consultam snapshots privados antes de decidir reiniciar; o painel usa prontidão corrente do relay, cujo snapshot agora inclui `character_id`. Configuração, comandos de testes e limitações da validação no Fly estão em [LOGIN_HEADLESS.md](../LOGIN_HEADLESS.md). O transporte padrão continua `standard` até validação real do candidato.
