# Envia o cookie do farm (contas do Formigueiro) para o Fly.io como segredo DD_COOKIE do app
# ddbot-formigueiro. O valor vai pela entrada padrão: não aparece na tela nem no histórico.
# Uso: botão direito > Executar com o PowerShell.
$ErrorActionPreference = "Stop"
$fly = Join-Path $env:USERPROFILE ".fly\bin\flyctl.exe"
$arquivo = "C:\Users\ATOS I\ddbot\codigo\run\sessao_atual.json"
$cookie = (Get-Content $arquivo -Raw | ConvertFrom-Json).cookie
if (-not $cookie -or $cookie.Length -lt 100) { throw "cookie não encontrado em $arquivo" }
# PowerShell 5.1 põe BOM no texto enviado por pipe e o Fly recusa o nome; quem escreve é o Python.
# Sem --stage: a máquina reinicia com o cookie novo (use quando o cookie expirar).
$env:DD_COOKIE_TMP = $cookie
py -3.12 -I -c "import os; print('DD_COOKIE=' + os.environ['DD_COOKIE_TMP'])" | & $fly secrets import -a ddbot-formigueiro
Remove-Item Env:DD_COOKIE_TMP
Write-Host "Cookie enviado ao Fly (segredo DD_COOKIE). Pode fechar esta janela."
Read-Host "Enter para fechar"
