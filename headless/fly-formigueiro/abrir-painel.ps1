# Abre o painel do Formigueiro no navegador padrao.
# 10/11: a contagem roda DENTRO do Fly (continua com o PC desligado). ver_paineis.py (sem janela) so baixa
# os painel.html prontos do Formigueiro e do Pintinho a cada 60 s.
$pasta = $PSScriptRoot
$pw = "C:\Users\ATOS I\AppData\Local\Programs\Python\Python312\pythonw.exe"
$rodando = Get-CimInstance Win32_Process -Filter "Name='pythonw.exe'" | ForEach-Object { $_.CommandLine }
if (-not ($rodando -like '*ver_paineis.py*')) {
    Start-Process -FilePath $pw -ArgumentList '"C:\Users\ATOS I\ddbot\ver_paineis.py"' -WorkingDirectory "C:\Users\ATOS I\ddbot" -WindowStyle Hidden
    Start-Sleep 5
}
Start-Process (Join-Path $pasta "painel.html")
