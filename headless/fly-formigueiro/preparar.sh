#!/bin/sh
# Monta fly-formigueiro/contexto/ com cópias do que a imagem precisa. Nenhum cookie entra:
# sessao*.json, run/ (cookie renovado, captchas, lua.tar) e chaves ficam de fora.
# Uso (Git Bash): sh preparar.sh   depois: cd contexto && flyctl deploy -c ../fly.toml
set -eu
AQUI=$(cd "$(dirname "$0")" && pwd)
DDBOT=$(dirname "$AQUI")
REPOS="/c/Users/ATOS I/source/repos/ddtank-context"
C="$AQUI/contexto"
rm -rf "$C" && mkdir -p "$C/codigo" "$C/secret" "$C/bot-ddtank-src"
cp -R "$DDBOT/codigo/api" "$DDBOT/codigo/bot" "$DDBOT/codigo/ddrelay" "$DDBOT/codigo/go.work" "$C/codigo/"
[ -f "$DDBOT/codigo/go.work.sum" ] && cp "$DDBOT/codigo/go.work.sum" "$C/codigo/"
cp -R "$REPOS/ddddocr/ddddocr" "$C/ddddocr"
cp -R "$REPOS/bot-ddtank/src/." "$C/bot-ddtank-src/"
find "$C" -name __pycache__ -type d -prune -exec rm -rf {} +
cp "$DDBOT/headless-bot.yaml" "$DDBOT/formigueiro-grupo.lua" "$DDBOT/ocr_worker.py" "$C/secret/"
cp "$DDBOT/pintinho/renovar_cookie.py" "$AQUI/iniciar.sh" "$C/secret/"
cp "$AQUI/Dockerfile" "$C/"
# Chave bot<->relay: trocada por __CHAVE__, sorteada a cada início. Bot lê Lua de /app/lua.
sed -E 's/^(\s*api_key:).*/\1 __CHAVE__/; s#^lua_dir:.*#lua_dir: /app/lua#' "$DDBOT/bot.yaml" > "$C/secret/bot.yaml"
sed -E 's/^(\s*api_key:).*/\1 __CHAVE__/' "$DDBOT/relay.yaml" > "$C/secret/relay.yaml"
mkdir -p "$C/painel" && cp "$AQUI/painel_fly.py" "$C/painel/painel.py" && cp "$AQUI/coletor.py" "$DDBOT/nomes-itens.json" "$C/painel/" && mv "$C/painel/nomes-itens.json" "$C/painel/nomes.json"
printf '**/sessao*.json\n**/*.antes-*\n' > "$C/.dockerignore"
if grep -rlE '"cookie"|\.Auth=' "$C" --include=*.json --include=*.yaml 2>/dev/null; then echo "ERRO: cookie no contexto"; exit 1; fi
echo "contexto pronto: $(du -sh "$C" | cut -f1)"
