#!/bin/sh
# Sondagem da Olimpíadas (contêiner ddbot-olimpiadas-sonda): bot + relay + logins, script = $SCRIPT.
set -u
SCRIPT=${SCRIPT:-sonda.lua}
LOGINS=${LOGINS:-"lider convidadas1 convidadas2"}
rm -rf /tmp/ddctl /tmp/cfg /tmp/lua /tmp/*.log && mkdir -p -m 700 /tmp/ddctl && mkdir -p /tmp/cfg || exit 1
cp /secret/bot.yaml /secret/olimpiadas/relay.yaml /secret/olimpiadas/*.yaml /tmp/cfg/ || exit 1
for s in sessao_lider sessao_conv1 sessao_conv2; do cp "/secret/olimpiadas/$s.json" /tmp/cfg/ && chmod 600 "/tmp/cfg/$s.json" || exit 1; done
cp "/secret/olimpiadas/$SCRIPT" /tmp/cfg/script.lua || exit 1
export DDBOT_API_KEY=$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')
tar -C /ws/bot -cf /tmp/lua.tar lua && tar -C /tmp -xf /tmp/lua.tar && cp -R /secret/olimpiadas/lua/. /tmp/lua/ || exit 1
cd /ws/bot && go build -o /tmp/ddbot ./cmd/ddbot || exit 1
cd /ws/ddrelay && go build -o /tmp/ddrelay ./cmd/ddrelay || exit 1
cd /tmp/cfg
/tmp/ddbot serve -config /tmp/cfg/bot.yaml > /tmp/bot.log 2>&1 &
esperar() { i=0; until grep -q "$1" "$2"; do i=$((i + 1)); [ $i -gt 900 ] && { echo "não iniciou: $2"; exit 1; }; sleep 0.2; done; }
esperar "bot listening" /tmp/bot.log
/tmp/ddrelay run -config /tmp/cfg/relay.yaml > /tmp/relay.log 2>&1 &
esperar "relay listening" /tmp/relay.log
for l in $LOGINS; do /tmp/ddrelay login -config /tmp/cfg/$l.yaml > /tmp/login-$l.log 2>&1 & done
tail -F /tmp/bot.log /tmp/relay.log /tmp/login-*.log
