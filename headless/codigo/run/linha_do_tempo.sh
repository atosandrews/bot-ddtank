#!/bin/bash
# Diagnóstico: grava mudanças de fase/motivo das 4 sessões com o tempo de sessão (session.now()).
saida=${1:-/ws/run/linha_do_tempo.txt}
duracao=${2:-900}
: > "$saida"
fim=$(( $(date +%s) + duracao ))
declare -A ultimo
while [ "$(date +%s)" -lt "$fim" ]; do
  for n in 1 2 3 4; do
    v=$(/tmp/ddrelay control -socket /tmp/ddctl/ddrelay.sock -session $n -eval 'local s = formigueiro_status and formigueiro_status() or {}; return string.format("%.1f|%d|%s|%d|%s", session.now(), player.state().id, tostring(s.phase), s.completed_runs or 0, string.sub(s.reason or "", 1, 90))' 2>/dev/null | grep -o '"value":"[^"]*"' | cut -d'"' -f4)
    chave="${v#*|}"
    if [ -n "$v" ] && [ "$chave" != "${ultimo[$n]:-}" ]; then echo "$v" >> "$saida"; ultimo[$n]="$chave"; fi
  done
  sleep 0.3
done
