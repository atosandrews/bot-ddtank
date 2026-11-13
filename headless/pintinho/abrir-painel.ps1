# Abre o painel do Pintinho no navegador padrao.
# 10/11: a contagem roda DENTRO do Fly (continua com o PC desligado). No PC ficam dois processos sem janela:
# ver_paineis.py (baixa os painel.html prontos do Formigueiro e do Pintinho a cada 60 s) e
# servir_painel.py (entrega so o painel.html em http://127.0.0.1:8765/).
$pasta = $PSScriptRoot
$pw = "C:\Users\ATOS I\AppData\Local\Programs\Python\Python312\pythonw.exe"
$rodando = Get-CimInstance Win32_Process -Filter "Name='pythonw.exe'" | ForEach-Object { $_.CommandLine }
if (-not ($rodando -like '*ver_paineis.py*')) {
    Start-Process -FilePath $pw -ArgumentList '"C:\Users\ATOS I\ddbot\ver_paineis.py"' -WorkingDirectory "C:\Users\ATOS I\ddbot" -WindowStyle Hidden
}
if (-not ($rodando -like '*servir_painel.py*')) {
    Start-Process -FilePath $pw -ArgumentList "`"$pasta\servir_painel.py`"" -WorkingDirectory $pasta -WindowStyle Hidden
    Start-Sleep 2
}
Start-Process "http://127.0.0.1:8765/"
