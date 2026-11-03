# Para o grupo e grava relatorio-<data>.txt com as partidas e os drops desde o início do farm.
# Usado pela tarefa agendada "DDBot parar 10h"; pode rodar à mão também.
$pasta = $PSScriptRoot
docker stop ddbot-grupo | Out-Null
& py -3.12 -I "$pasta\painel.py" --relatorio
