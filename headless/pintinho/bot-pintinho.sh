#!/bin/sh
# Instância Pintinho, separada do Formigueiro: tudo vem de /secret/pintinho/ (pasta ddbot\pintinho)
# e o que precisa persistir (pacote Lua, cookie renovado) fica no volume /cache (ddbot-pintinho-cache).
# Do código do Formigueiro (/ws, montado só leitura) usa apenas o bot Go, o ddrelay e a biblioteca Lua
# original; os arquivos Lua do Pintinho são copiados por cima em /tmp/lua, sem tocar em /ws.
set -u
SCRIPT=${SCRIPT:-pintinho-forte.lua}
rm -rf /tmp/ddctl /tmp/cfg /tmp/lua /tmp/*.log && mkdir -p -m 700 /tmp/ddctl && mkdir -p /tmp/cfg && cp /secret/pintinho/sessao.json /secret/pintinho/headless-bot.yaml /secret/pintinho/convidadas.yaml /secret/bot.yaml /tmp/cfg/ || exit 1
# Convidadas: cookie do farm, o mais novo entre o salvo à mão, o renovado pelo Formigueiro e o renovado aqui.
cp /secret/sessao.json /tmp/cfg/sessao_farm.json || exit 1
for c in /ws/run/sessao_atual.json /cache/sessao_farm.json; do
  if [ -f "$c" ] && [ "$c" -nt /tmp/cfg/sessao_farm.json ]; then cp "$c" /tmp/cfg/sessao_farm.json || exit 1; fi
done
chmod 600 /tmp/cfg/sessao_farm.json || exit 1
cp /secret/pintinho/relay.yaml /tmp/cfg/relay.yaml && cp "/secret/pintinho/$SCRIPT" /tmp/cfg/script.lua || exit 1
if [ -f /cache/sessao_pintinho.json ] && [ /cache/sessao_pintinho.json -nt /secret/pintinho/sessao.json ]; then
  cp /cache/sessao_pintinho.json /tmp/cfg/sessao.json || exit 1
fi
chmod 600 /tmp/cfg/sessao.json || exit 1
export DDBOT_API_KEY=$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')
if [ ! -f /cache/lua.tar ] || [ -n "$(find /ws/bot/lua -newer /cache/lua.tar -print -quit)" ]; then
  tar -C /ws/bot -cf /cache/lua.tar.novo lua && mv /cache/lua.tar.novo /cache/lua.tar || exit 1
fi
tar -C /tmp -xf /cache/lua.tar || exit 1
cp -R /secret/pintinho/lua/. /tmp/lua/ || exit 1
cd /ws/bot && go build -o /tmp/ddbot ./cmd/ddbot || exit 1
cd /ws/ddrelay && go build -o /tmp/ddrelay ./cmd/ddrelay || exit 1
cd /tmp/cfg
/tmp/ddbot serve -config /tmp/cfg/bot.yaml > /tmp/bot.log 2>&1 &
esperar() { i=0; until grep -q "$1" "$2"; do i=$((i + 1)); [ $i -gt 900 ] && { echo "não iniciou: $2"; cat "$2"; exit 1; }; sleep 0.2; done; }
esperar "bot listening" /tmp/bot.log
/tmp/ddrelay run -config /tmp/cfg/relay.yaml > /tmp/relay.log 2>&1 &
esperar "relay listening" /tmp/relay.log
/tmp/ddrelay login -config /tmp/cfg/headless-bot.yaml > /tmp/login.log 2>&1 &
/tmp/ddrelay login -config /tmp/cfg/convidadas.yaml > /tmp/login-convidadas.log 2>&1 &
tail -F /tmp/bot.log /tmp/relay.log /tmp/login.log /tmp/login-convidadas.log &
SEM_PROGRESSO=${SEM_PROGRESSO:-300}
PERSONAGENS=$(cat /tmp/cfg/headless-bot.yaml /tmp/cfg/convidadas.yaml | grep -c '^  - name:')
# Toda consulta ao relay tem prazo: um comando preso não pode travar o vigia.
ctl() { timeout 15 /tmp/ddrelay control -socket /tmp/ddctl/ddrelay.sock "$@"; }
conectados() {
  ctl -sessions 2>/dev/null \
    | python3 -I -c 'import json,sys; v=json.load(sys.stdin).get("value") or []; print(" ".join(str(s["session"]) for s in v if s.get("game_ready")))' 2>/dev/null
}
# 08/11: cada sessão aceita no máximo 4096 consultas (-eval) enquanto vive (bot/internal/session/scripts.go,
# maxScriptRequests; o registro nunca esvazia). Com 4 consultas por personagem a cada 20 s, mais o painel, a
# fila da principal encheu em ~3 h e o vigia e o painel ficaram cegos para ela. Agora o vigia faz UMA consulta
# por personagem por volta e, se alguma fila encher, reinicia o grupo entre partidas.
# Linha por personagem em /tmp/vigia.txt: sessão|partidas|mapa|turnos|sem_confirmação|motivo (ou sessão|cheio).
CONSULTA='local g = grupo_status and grupo_status() or {}; local s = pintinho_status and pintinho_status() or {}; local t = 0; for _, v in pairs(s.my_turns or {}) do t = t + v end; local c = 0; for _, r in pairs(g.recuperacoes or {}) do if tostring(r):find("confirmation_missing", 1, true) then c = c + 1 end end; return tostring((g.total or {}).runs) .. "|" .. tostring(s.map_id) .. "|" .. t .. "|" .. c .. "|" .. tostring(s.reason or "")'
coletar() {
  : > /tmp/vigia.txt
  for n in $(conectados); do
    ctl -session $n -eval "$CONSULTA" 2>/dev/null | python3 -I -c '
import json, sys
try: r = json.load(sys.stdin)
except Exception: r = {}
e = json.dumps(r.get("error") or "")
print(sys.argv[1] + "|" + ("cheio" if "request_limit" in e else str(r.get("value", "")).replace("\n", " ")))' "$n" >> /tmp/vigia.txt
  done
}
# Progresso = partidas, estágio e turnos (sem a fase: presa no convite ela alterna configurando/pausada).
progresso() { awk -F'|' '$2 != "cheio" {printf "%s/%s/%s ", $2, $3, $4}' /tmp/vigia.txt; }
# Em partida = algum personagem está num estágio (map_id). Fora disso (sala, convite, tela inicial)
# por SEM_PARTIDA segundos, o grupo reinicia: a partida sempre começa em menos de 1 min.
SEM_PARTIDA=${SEM_PARTIDA:-300}
em_partida() { awk -F'|' '$2 != "cheio" && $3 != "" && $3 != "nil" {achou = 1} END {exit !achou}' /tmp/vigia.txt; }
# Convidada que caiu e reconectou volta à tela inicial esperando convite, e a principal fica presa
# esperando os 4 na sala: o jogo não sai disso sozinho.
esperando_sala() { grep -qE "room members|invited character" /tmp/vigia.txt; }
# 07/11 19:03: a conexão da principal caiu e voltou em 6 s, mas o convite seguinte ficou sem confirmação do
# servidor e o grupo parou 7 min até o vigia de partida agir. Cada falha dessas vira uma recuperação
# "confirmation_missing" (uma a cada ~50 s); duas novas em até 3 min reiniciam o grupo.
sem_confirmacao() { awk -F'|' '$2 != "cheio" {t += $5} END {print t + 0}' /tmp/vigia.txt; }
fila_cheia() { grep -q '|cheio$' /tmp/vigia.txt; }
faltando=0; ultimo=-1; desde=$(date +%s); sala=0; partida=$(date +%s); cheia=0
confirmacoes=0; confirmacoes_hora=$(date +%s)
RENOVAR_COOKIE=${RENOVAR_COOKIE:-21600}
renovar() {
  timeout 120 python3 -I /secret/pintinho/renovar_cookie.py /tmp/cfg/sessao.json /cache/sessao_pintinho.json
  timeout 120 python3 -I /secret/pintinho/renovar_cookie.py /tmp/cfg/sessao_farm.json /cache/sessao_farm.json
}
sleep 20 && renovar
renovado=$(date +%s)
# 07/11: o usuário quer os 4 sempre juntos e falha corrigida na hora: o vigia confere a cada 20 s.
while sleep 20; do
  if [ $(( $(date +%s) - renovado )) -ge "$RENOVAR_COOKIE" ]; then renovar; renovado=$(date +%s); fi
  for p in "ddbot serve" "ddrelay run" "ddrelay login -config /tmp/cfg/headless-bot" "ddrelay login -config /tmp/cfg/convidadas"; do
    if ! pgrep -f "/tmp/$p" > /dev/null; then echo "vigia: processo '$p' parou; reiniciando o grupo"; exit 1; fi
  done
  online=$(conectados | wc -w)
  if [ "$online" -lt "$PERSONAGENS" ]; then faltando=$((faltando + 1)); else faltando=0; fi
  if [ "$faltando" -ge 3 ]; then echo "vigia: só $online de $PERSONAGENS personagens conectados há 1 min; reiniciando o grupo"; exit 1; fi
  coletar
  if fila_cheia; then
    [ "$cheia" = 0 ] && { cheia=$(date +%s); echo "vigia: fila de consultas cheia; reinicio na próxima troca de partida"; }
    if ! em_partida || [ $(( $(date +%s) - cheia )) -ge 900 ]; then echo "vigia: fila de consultas cheia; reiniciando o grupo entre partidas"; exit 1; fi
  fi
  if esperando_sala; then sala=$((sala + 1)); else sala=0; fi
  if [ "$sala" -ge 6 ]; then echo "vigia: principal esperando convidada/sala há 2 min; reiniciando o grupo"; exit 1; fi
  c=$(sem_confirmacao)
  if [ "$c" -le "$confirmacoes" ] || [ $(( $(date +%s) - confirmacoes_hora )) -ge 180 ]; then confirmacoes=$c; confirmacoes_hora=$(date +%s)
  elif [ $(( c - confirmacoes )) -ge 2 ]; then echo "vigia: comando do grupo sem confirmação do servidor ($((c - confirmacoes))x); reiniciando o grupo"; exit 1; fi
  if em_partida; then partida=$(date +%s); fi
  if [ $(( $(date +%s) - partida )) -ge "$SEM_PARTIDA" ]; then echo "vigia: nenhuma partida em andamento há ${SEM_PARTIDA}s; reiniciando o grupo"; exit 1; fi
  [ "$SEM_PROGRESSO" = 0 ] && continue
  agora=$(progresso)
  if [ "$agora" != "$ultimo" ]; then ultimo=$agora; desde=$(date +%s); fi
  if [ $(( $(date +%s) - desde )) -ge "$SEM_PROGRESSO" ]; then echo "vigia: nenhum turno, estágio ou partida novo em ${SEM_PROGRESSO}s; reiniciando o grupo"; exit 1; fi
done
