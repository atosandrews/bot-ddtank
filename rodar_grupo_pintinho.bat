@echo off
rem Duplo clique para rodar o Pintinho em grupo: a principal faz os estagios 1 a 3 sozinha
rem e chama as outras contas so para o estagio 4. Abra todas as contas antes; ao iniciar,
rem clique na janela da principal.
cd /d "%~dp0"

call preparar_ambiente.bat
if errorlevel 1 (
    pause
    exit /b 1
)

".venv\Scripts\python.exe" -m src.grupo_pintinho %*
pause
