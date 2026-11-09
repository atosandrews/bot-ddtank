# Sobe a instância Pintinho (Difícil) no contêiner próprio ddbot-pintinho, separada do Formigueiro.
# Conta forte: pintinho\sessao.json + pintinho\headless-bot.yaml. O código do Formigueiro é montado
# só leitura; nada dele é alterado. Para parar: parar-pintinho.ps1.
$ErrorActionPreference = "Stop"
$pintinho = $PSScriptRoot
$pasta = Split-Path $pintinho -Parent
$codigo = Join-Path $pasta "codigo"
$ddddocr = "C:\Users\ATOS I\source\repos\ddtank-context\ddddocr"
$botDdtank = "C:\Users\ATOS I\source\repos\ddtank-context\bot-ddtank"
docker rm -f ddbot-pintinho *> $null
# CAPTCHA_BANCO: imagens de captcha no volume do Pintinho (/ws, do Formigueiro, é só leitura).
docker run -d --name ddbot-pintinho --restart unless-stopped `
    -e CAPTCHA_BANCO=/cache/captchas `
    -v ddbot-gocache:/root/.cache/go-build `
    -v ddbot-gomod:/go/pkg/mod `
    -v ddbot-pintinho-cache:/cache `
    -v "${codigo}:/ws:ro" `
    -v "${pasta}:/secret:ro" `
    -v "${ddddocr}:/ddddocr:ro" `
    -v "${botDdtank}:/bot-ddtank:ro" `
    ddbot-build sh /secret/pintinho/bot-pintinho.sh | Out-Null
Write-Host "Pintinho iniciado (contêiner ddbot-pintinho)."
