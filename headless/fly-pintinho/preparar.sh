#!/bin/sh
# Monta fly-pintinho/contexto/ com cópias do que a imagem precisa. Nenhum cookie entra.
# Uso (Git Bash): sh preparar.sh   depois: cd contexto && flyctl deploy
set -eu
AQUI=$(cd "$(dirname "$0")" && pwd)
DDBOT=$(dirname "$AQUI")
P="$DDBOT/pintinho"
REPOS="/c/Users/ATOS I/source/repos/ddtank-context"
C="$AQUI/contexto"
rm -rf "$C" && mkdir -p "$C/codigo" "$C/secret" "$C/bot-ddtank-src" "$C/lua-pintinho"
cp -R "$DDBOT/codigo/api" "$DDBOT/codigo/bot" "$DDBOT/codigo/ddrelay" "$DDBOT/codigo/go.work" "$C/codigo/"
cp -R "$REPOS/ddddocr/ddddocr" "$C/ddddocr"
cp -R "$REPOS/bot-ddtank/src/." "$C/bot-ddtank-src/"
cp -R "$P/lua/." "$C/lua-pintinho/"
find "$C" -name __pycache__ -type d -prune -exec rm -rf {} +
find "$C/lua-pintinho" -name "*.antes-*" -delete
cp "$P/headless-bot.yaml" "$P/convidadas.yaml" "$P/convidadas2.yaml" "$P/relay.yaml" "$P/pintinho-forte.lua" \
   "$P/renovar_cookie.py" "$DDBOT/bot.yaml" "$DDBOT/ocr_worker.py" "$AQUI/iniciar.sh" "$C/secret/"
cp "$AQUI/Dockerfile" "$AQUI/fly.toml" "$C/"
mkdir -p "$C/painel" && cp "$P/painel_pintinho.py" "$C/painel/painel.py" && cp "$P/coletor_fly.py" "$C/painel/" && cp "$DDBOT/nomes-itens.json" "$C/painel/nomes.json"
printf '**/sessao*.json\n**/*.antes-*\n' > "$C/.dockerignore"
if grep -rlE '"cookie"|\.Auth=' "$C" --include=*.json --include=*.yaml 2>/dev/null; then echo "ERRO: cookie no contexto"; exit 1; fi
echo "contexto pronto: $(du -sh "$C" | cut -f1)"
