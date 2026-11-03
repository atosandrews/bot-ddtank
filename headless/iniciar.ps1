# Sobe o Formigueiro em grupo (bot + relay + 4 personagens) no Docker e o painel ao vivo.
# Uso: clique com o botão direito > Executar com o PowerShell, ou: powershell -File iniciar.ps1
$ErrorActionPreference = "Stop"
$pasta = $PSScriptRoot
$codigo = Join-Path $pasta "codigo"
$ddddocr = "C:\Users\ATOS I\source\repos\ddtank-context\ddddocr"
$botDdtank = "C:\Users\ATOS I\source\repos\ddtank-context\bot-ddtank"

docker image inspect ddbot-build *> $null
if ($LASTEXITCODE -ne 0) {
    Write-Host "Construindo a imagem ddbot-build (primeira vez)..."
    docker build -t ddbot-build -f (Join-Path $codigo "Dockerfile") (Join-Path $codigo "api\proto")
}

docker rm -f ddbot-grupo *> $null
# Volumes de cache do Go: a partir da segunda vez a compilação leva segundos, não ~1 minuto.
# --restart unless-stopped: o Docker reinicia o grupo quando o vigia sai (travou, caiu ou foi
# desconectado) e quando o Docker Desktop abre de novo; parar.ps1 encerra de vez.
docker run -d --name ddbot-grupo --restart unless-stopped `
    -v ddbot-gocache:/root/.cache/go-build `
    -v ddbot-gomod:/go/pkg/mod `
    -v "${codigo}:/ws" `
    -v "${pasta}:/secret:ro" `
    -v "${ddddocr}:/ddddocr:ro" `
    -v "${botDdtank}:/bot-ddtank:ro" `
    ddbot-build sh /ws/run/bot-all.sh | Out-Null
Write-Host "Grupo iniciado (contêiner ddbot-grupo)."

Start-Process py -ArgumentList "-3.12", "-I", "`"$pasta\painel.py`"", "--ao-vivo"
Start-Process (Join-Path $pasta "painel.html")
