#!/bin/sh
# Formigueiro no Fly.io. Igual ao codigo/run/bot-all.sh do PC, com três diferenças:
# - binários e Lua já vêm prontos na imagem (/app);
# - cookie: o renovado no volume (/data/sessao_atual.json) ou, na primeira vez, o segredo DD_COOKIE;
# - a chave entre bot e relay é sorteada a cada início (os dois só conversam dentro da máquina).
# Sair daqui reinicia a máquina (restart "always" no fly.toml) e faz login de novo.
set -u
rm -rf /tmp/ddctl /tmp/cfg && mkdir -p -m 700 /tmp/ddctl && mkdir -p /tmp/cfg /data/captchas || exit 1
cp /secret/headless-bot.yaml /secret/formigueiro-grupo.lua /tmp/cfg/ || exit 1
# Um DD_COOKIE novo (enviar-cookie.ps1) vale mais que o renovado no disco; o mesmo de antes, não.
ASSINATURA=$(printf '%s' "${DD_COOKIE:-}" | sha256sum | cut -c1-16)
if [ -n "${DD_COOKIE:-}" ] && [ "$ASSINATURA" != "$(cat /data/cookie_segredo.txt 2>/dev/null)" ]; then
  python3 -I -c 'import json, os; json.dump({"cookie": os.environ["DD_COOKIE"]}, open("/tmp/cfg/sessao.json", "w"))' || exit 1
  rm -f /data/sessao_atual.json; echo "$ASSINATURA" > /data/cookie_segredo.txt
elif [ -s /data/sessao_atual.json ]; then
  cp /data/sessao_atual.json /tmp/cfg/sessao.json || exit 1
else
  echo "sem cookie: defina o segredo DD_COOKIE (fly secrets set DD_COOKIE=...)"; sleep 300; exit 1
fi
chmod 600 /tmp/cfg/sessao.json || exit 1
CHAVE=$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')
sed "s/__CHAVE__/$CHAVE/" /secret/bot.yaml > /tmp/cfg/bot.yaml && sed "s/__CHAVE__/$CHAVE/" /secret/relay.yaml > /tmp/cfg/relay.yaml || exit 1
chmod 600 /tmp/cfg/bot.yaml /tmp/cfg/relay.yaml
export DDBOT_API_KEY=$CHAVE

cd /tmp/cfg
/app/ddbot serve -config /tmp/cfg/bot.yaml > /tmp/bot.log 2>&1 &
esperar() { i=0; until grep -q "$1" "$2"; do i=$((i + 1)); [ $i -gt 900 ] && { echo "não iniciou: $2"; cat "$2"; exit 1; }; sleep 0.2; done; }
esperar "bot listening" /tmp/bot.log
/app/ddrelay run -config /tmp/cfg/relay.yaml > /tmp/relay.log 2>&1 &
esperar "relay listening" /tmp/relay.log
/app/ddrelay login -config /tmp/cfg/headless-bot.yaml > /tmp/login.log 2>&1 &
tail -F /tmp/bot.log /tmp/relay.log /tmp/login.log &
# 10/11: painel e contagem do farm rodam aqui dentro (o PC pode desligar); ficam em /data/painel.
# Hora de Brasília (UTC-3, sem horário de verão). O PC só baixa /data/painel/painel.html.
PAINEL_MAQUINA=/data/painel TZ='<-03>3' nice -n 10 python3 -I /app/painel/painel.py --ao-vivo > /tmp/painel.log 2>&1 &

SEM_PROGRESSO=${SEM_PROGRESSO:-420}
PERSONAGENS=$(grep -c '^  - name:' /tmp/cfg/headless-bot.yaml)
ctl() { timeout 15 /app/ddrelay control -socket /tmp/ddctl/ddrelay.sock "$@"; }
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
renovar() { timeout 120 python3 -I /secret/renovar_cookie.py /tmp/cfg/sessao.json /data/sessao_atual.json; }
faltando=0; ultimo=-1; desde=$(date +%s)
RENOVAR_COOKIE=${RENOVAR_COOKIE:-21600}
sleep 20 && renovar
renovado=$(date +%s)
while sleep 60; do
  if [ $(( $(date +%s) - renovado )) -ge "$RENOVAR_COOKIE" ]; then renovar; renovado=$(date +%s); fi
  for p in "ddbot serve" "ddrelay run" "ddrelay login"; do
    if ! pgrep -f "/app/$p" > /dev/null; then echo "vigia: processo '$p' parou; reiniciando o grupo"; exit 1; fi
  done
  online=$(conectados | wc -w)
  if [ "$online" -lt "$PERSONAGENS" ]; then faltando=$((faltando + 1)); else faltando=0; fi
  if [ "$faltando" -ge 2 ]; then echo "vigia: só $online de $PERSONAGENS personagens conectados; reiniciando o grupo"; exit 1; fi
  agora=$(partidas)
  # resumo para o painel: fly ssh console -C "cat /tmp/resumo.txt"
  echo "$(date '+%d/%m %H:%M:%S') online=$online partidas=$agora" > /tmp/resumo.txt
  if [ "$agora" != "$ultimo" ]; then ultimo=$agora; desde=$(date +%s); fi
  if [ $(( $(date +%s) - desde )) -ge "$SEM_PROGRESSO" ]; then echo "vigia: nenhuma partida terminou em ${SEM_PROGRESSO}s; reiniciando o grupo"; exit 1; fi
done
