# Pintinho Corre Rápido (Difícil) headless

Instância separada do Formigueiro (pedido do usuário, 05/11/2026): **nenhum arquivo do Formigueiro é
alterado**. Tudo do Pintinho fica nesta pasta; o contêiner `ddbot-pintinho` monta o `codigo` só
leitura e copia `pintinho/lua/` por cima da biblioteca Lua em `/tmp/lua`.

- `lua/instances/pintinho.lua`: motor gerado por `fazer_pintinho.py` a partir do `formigueiro.lua`
  (instância 7, Difícil = 2, mira pelo terreno exato, tridente no estágio 1, alvos por nome).
- `lua/data/maps.lua` (cópia com `terrain=` nos mapas do Pintinho) e `lua/data/terrain/116x.lua`
  (gerados por `terreno.py` do dead.png do cache; alpha > 150, mesmo método que reproduz `1120.lua`).
  Hashes: 1161 `ef7600e6…888f`, 1163 `632bc650…5b3b`.
- `pintinho-solo.lua`: script do relay (a conta forte). `pintinho-grupo.lua`/`sonda.lua`: sondagem.
- `bot-pintinho.sh`, `relay.yaml`, `renovar_cookie.py` (cookie em `/cache`, volume `ddbot-pintinho-cache`).
- `iniciar-pintinho.ps1` / `parar-pintinho.ps1`.
- Conta (não versionar): `sessao.json` (`{"cookie": "<.Auth>"}`) e `headless-bot.yaml`
  (modelo em `headless-bot.exemplo.yaml`).

## Fluxo atual (05/11/2026, testado no jogo)

Srmaniaco (ID no jogo 14065; login próprio em `sessao.json`) lidera e é a
única que ataca. fakeddt1, eliseu e Guestf95c4 (sessão do farm, `convidadas.yaml`) ficam na sala a
partida inteira e passam a vez na hora (`attack_policy = "leader"`, `party.lua` original só leitura).

| Estágio | Mapa | Alvos | Regra |
|---|---|---|---|
| 1 | 1161 | Pintinho sem Casca (4000 HP) | 2 + 3 + 9 (+2, tridente, sobra em dano); mira no **maior grupo** (vizinhos a até 100 px), grudado primeiro |
| 2 | 1162 | galinhas nas 3 prateleiras | 2 + 9 (+2 e sobra em dano), sem tridente; mira exata |
| 3 | 1163 | Galinheiro (75 000 HP) + galos/galinhas que renascem | **Tática definitiva (09/11, aprovada pelo usuário):** turno 1 **voa** (Voar 10016); turno 2 **anda uma vez só** para longe da casa até o ponto a 342 px dela (x≈680; até 120 px, guardando 110 de energia para o +2); turno 3+ (`full_maps`) **sempre** 2 + 3 + 9 (tridente, +2, dano máximo) no Galinheiro, sem andar, sem mirar em galinha e mesmo sem tiro limpo. Medido em 9 partidas: 7 turnos, ~1:45, vida final > 12 mil, sem derrota. Não afastar mais da casa: longe dela nascem galinhas novas. Backup da versão anterior: `lua/instances/pintinho.lua.antes-full-sempre` |
| 4 | 1164 | Burrinho (46 000) e Gaiola de Galinha (100) | 2 + 9 (+2 e sobra em dano), sem tridente; Burrinho antes, Gaiola por último |

Partida completa: ~9 min (06/11: estágios 1,9 + 1,7 + 2,1 + 2,4 min; 8/4/6/5 turnos), recomeça sozinha. A tecla 9 do jogo = gastar a sobra de energia em dano (damage_props). O servidor aceita andar mas não devolve a posição a quem andou: o motor mira do destino. Voo, andar e itens recusados por stale_state são reenviados. Fatia de mira 90 ms.
Medição 17:23–18:03: 3 vitórias e 2 derrotas (uma no estágio 1 lento, 9,7 min); sem o voo o estágio 3 travava. Não rodar junto com o Formigueiro.
`pintinho_party.lua` (convite só no estágio 4) ficou sem uso: as convidadas que entram no meio não
recebem a lista de jogadores da luta e não conseguem passar a vez. 

## Sondagem de 05/11/2026 (contas headless, sem cliente)

| Estágio | Mapa | Monstros | Terreno |
|---|---|---|---|
| 1 | 1161 | "Pintinho sem Casca" (4000 HP), em ondas até 30 | `data/terrain/1161.lua` |
| 2 | 1162 | ainda não visto (carregando quando a sondagem parou) | **falta** |
| 3 | ? (provável 1163, fazenda com a casa) | ? | `1163.lua` pronto |
| 4 | ? | ? | **falta** |

O grupo venceu o estágio 1 sozinho em ~5 min (carta de ouro: 1072). No 1162 o motor pausou
por falta de terreno, como deve (não chuta física).

## Falta

O `dead.png` dos mapas 1162 e do estágio 4 não está no cache do DDClassico, e o servidor de
recursos bloqueia download direto (Cloudflare). Para obtê-los: jogar **um** Pintinho até o
estágio 4 no cliente DDClassico; o cliente baixa os mapas para o cache e o terreno é gerado
com `terreno.py` (alpha > 150, mesmo método que reproduz `1120.lua` byte a byte).

## Vigia (reinício automático do grupo)

O `bot-pintinho.sh` confere o grupo a cada 60 s e reinicia o contêiner inteiro (o Docker sobe de novo sozinho) quando:

- algum processo (bot, relay, logins) parou;
- faltam personagens conectados por 2 verificações seguidas;
- a principal espera convidada ou membros da sala por 2 min;
- **nenhuma partida em andamento por 5 min** (`SEM_PARTIDA=300`): fora de estágio, na sala ou na tela inicial;
- nenhum turno, estágio ou partida novo por 5 min (`SEM_PROGRESSO=300`). A fase não conta como avanço, porque presa no convite ela alterna entre configurando e pausada.

Toda consulta ao relay tem prazo de 15 s e a renovação do cookie tem prazo de 120 s, para o vigia nunca travar.

## Tempo de reação (06/11)

- `followup_delay = 0.3` em `lua/instances/pintinho.lua`: espera antes de cada tiro extra do +1/+2 (era 1 s fixo, herdado do Formigueiro). Medido: tiros extras a 0,7 s um do outro (antes 1,4 s), servidor aceitou 3 de 3 em todos os turnos.
- Primeiro tiro sai 0,7–1,3 s depois do início da vez (3 itens confirmados um a um + mira).
- `pintinho_status().reactions` guarda os últimos 40 tempos ("tiro", "extra", "fim do turno: N tiro(s) aceitos").
