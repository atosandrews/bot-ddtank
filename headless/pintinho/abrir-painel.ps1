# Liga o painel do Pintinho (se ainda não estiver rodando) e abre no navegador padrão.
# Processos sem janela: painel_pintinho.py --ao-vivo (gera painel.html a cada 15 s e mantém o PC acordado)
# e servir_painel.py (entrega só o painel.html em http://127.0.0.1:8765/).
$pasta = $PSScriptRoot
$pw = "C:\Users\ATOS I\AppData\Local\Programs\Python\Python312\pythonw.exe"
$rodando = Get-CimInstance Win32_Process -Filter "Name='pythonw.exe'" | ForEach-Object { $_.CommandLine }
if (-not ($rodando -like '*painel_pintinho.py*')) {
    Start-Process -FilePath $pw -ArgumentList "`"$pasta\painel_pintinho.py`"", '--ao-vivo' -WorkingDirectory $pasta -WindowStyle Hidden
}
if (-not ($rodando -like '*servir_painel.py*')) {
    Start-Process -FilePath $pw -ArgumentList "`"$pasta\servir_painel.py`"" -WorkingDirectory $pasta -WindowStyle Hidden
    Start-Sleep 2
}
Start-Process "http://127.0.0.1:8765/"
