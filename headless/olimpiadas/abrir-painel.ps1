# Abre o painel da Olimpíadas no navegador (o ver_paineis.py, sem janela, baixa o painel.html do Fly a cada 60 s).
$pw = "C:\Users\ATOS I\AppData\Local\Programs\Python\Python312\pythonw.exe"
$rodando = Get-CimInstance Win32_Process -Filter "Name='pythonw.exe'" | ForEach-Object { $_.CommandLine }
if (-not ($rodando -like '*ver_paineis.py*')) {
    Start-Process -FilePath $pw -ArgumentList '"C:\Users\ATOS I\ddbot\ver_paineis.py"' -WorkingDirectory "C:\Users\ATOS I\ddbot" -WindowStyle Hidden
    Start-Sleep 30
}
Start-Process "$PSScriptRoot\painel.html"
