#!/bin/sh
set -u
rm -rf /tmp/ddctl /tmp/cfg /tmp/lua /tmp/*.log && mkdir -p -m 700 /tmp/ddctl && mkdir -p /tmp/cfg && cp /secret/sessao.json /secret/headless-bot.yaml /secret/bot.yaml /secret/relay.yaml /secret/formigueiro-grupo.lua /tmp/cfg/ || exit 1
# Cookie: usa o renovado automaticamente (run/sessao_atual.json) se for mais novo que o salvo à mão.
if [ -f /ws/run/sessao_atual.json ] && [ /ws/run/sessao_atual.json -nt /secret/sessao.json ]; then
  cp /ws/run/sessao_atual.json /tmp/cfg/sessao.json || exit 1
fi
chmod 600 /tmp/cfg/sessao.json || exit 1
export DDBOT_API_KEY=$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')

# Ler os ~13 mil arquivos Lua pela pasta do Windows leva 25-50 s; um .tar único lê em instantes.
# O pacote é refeito só quando algum arquivo Lua for mais novo que ele.
if [ ! -f /ws/run/lua.tar ] || [ -n "$(find /ws/bot/lua -newer /ws/run/lua.tar -print -quit)" ]; then
  tar -C /ws/bot -cf /ws/run/lua.tar lua || exit 1
fi
tar -C /tmp -xf /ws/run/lua.tar || exit 1

cd /ws/bot && go build -o /tmp/ddbot ./cmd/ddbot || exit 1
cd /ws/ddrelay && go build -o /tmp/ddrelay ./cmd/ddrelay || exit 1
cd /tmp/cfg
/tmp/ddbot serve -config /tmp/cfg/bot.yaml > /tmp/bot.log 2>&1 &
esperar() { i=0; until grep -q "$1" "$2"; do i=$((i + 1)); [ $i -gt 900 ] && { echo "não iniciou: $2"; cat "$2"; exit 1; }; sleep 0.2; done; }
esperar "bot listening" /tmp/bot.log
/tmp/ddrelay run -config /tmp/cfg/relay.yaml > /tmp/relay.log 2>&1 &
esperar "relay listening" /tmp/relay.log
# Keep the existing transport until the candidate passes a real authentication check.
AUTH_TRANSPORT=${AUTH_TRANSPORT:-standard}
AUTH_COORDINATION_DIR=${AUTH_COORDINATION_DIR:-/tmp/dd-auth-coordination}
/tmp/ddrelay login -config /tmp/cfg/headless-bot.yaml -transport "$AUTH_TRANSPORT" -status-file /tmp/ddctl/auth-main.json -coordination-dir "$AUTH_COORDINATION_DIR" > /tmp/login.log 2>&1 &
tail -F /tmp/bot.log /tmp/relay.log /tmp/login.log &

# Vigia: com o contêiner em --restart unless-stopped, sair daqui reinicia tudo (login novo).
# Sai se bot, relay ou login morrerem, ou se nenhuma partida terminar em SEM_PROGRESSO segundos
# (uma partida leva ~2,6 min; 7 min sem nenhuma é sinal de grupo travado).
SEM_PROGRESSO=${SEM_PROGRESSO:-420}
PERSONAGENS=$(grep -c '^  - name:' /tmp/cfg/headless-bot.yaml)
ctl() { timeout 15 /tmp/ddrelay control -socket /tmp/ddctl/ddrelay.sock "$@"; }
controle_saude() { ctl -sessions > /dev/null 2>&1; }
conectados() {
  ctl -sessions 2>/dev/null \
    | python3 -I -c 'import json,sys; v=json.load(sys.stdin).get("value") or []; print(" ".join(str(s["session"]) for s in v if s.get("game_ready")))' 2>/dev/null
}
partidas() {
  total=0
  for n in $(conectados); do
    v=$(ctl -session $n -eval 'return grupo_status and grupo_status().total.runs or 0' 2>/dev/null | grep -o '"value":[0-9]*' | cut -d: -f2)
    total=$(( total + ${v:-0} ))
  done
  echo "$total"
}
faltando=0
ultimo=-1
desde=$(date +%s)
# O cookie vale 14 dias a partir do último uso; renovar a cada 6 h o mantém válido para sempre.
RENOVAR_COOKIE=${RENOVAR_COOKIE:-21600}
sleep 20 && python3 -I /ws/run/renovar_cookie.py
renovado=$(date +%s)
while sleep 60; do
  if [ $(( $(date +%s) - renovado )) -ge "$RENOVAR_COOKIE" ]; then
    python3 -I /ws/run/renovar_cookie.py
    renovado=$(date +%s)
  fi
  for p in "ddbot serve" "ddrelay run" "ddrelay login"; do
    if ! pgrep -f "/tmp/$p" > /dev/null; then echo "vigia: processo '$p' parou; reiniciando o grupo"; exit 1; fi
  done
  if ! controle_saude; then echo "vigia: controle do relay indisponível; reiniciando o grupo"; exit 1; fi
  python3 -I /ws/run/auth_watchdog.py /tmp/ddctl/auth-main.json
  auth_estado=$?
  if [ "$auth_estado" -ne 0 ] && [ "$auth_estado" -ne 1 ]; then echo "vigia: estado de autenticação ausente, vencido ou travado; reiniciando o grupo"; exit 1; fi
  if [ "$auth_estado" -eq 0 ]; then
    echo "vigia: autenticação aguardando recuperação; preservando as sessões conectadas"
    faltando=0; ultimo=-1; desde=$(date +%s)
    continue
  fi
  # Um personagem que cai sozinho (ex.: a conta entrou por outro lugar) deixa o grupo esperando o
  # convite dele; dois minutos seguidos sem os PERSONAGENS conectados reiniciam o login de todos.
  online=$(conectados | wc -w)
  if [ "$online" -lt "$PERSONAGENS" ]; then faltando=$((faltando + 1)); else faltando=0; fi
  if [ "$faltando" -ge 2 ]; then
    echo "vigia: só $online de $PERSONAGENS personagens conectados; reiniciando o grupo"; exit 1
  fi
  agora=$(partidas)
  if [ "$agora" != "$ultimo" ]; then ultimo=$agora; desde=$(date +%s); fi
  if [ $(( $(date +%s) - desde )) -ge "$SEM_PROGRESSO" ]; then
    echo "vigia: nenhuma partida terminou em ${SEM_PROGRESSO}s; reiniciando o grupo"; exit 1
  fi
done
