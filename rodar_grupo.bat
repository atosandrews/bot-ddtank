@echo off
rem Duplo clique para rodar o bot com varias contas (principal + convidadas).
rem Abra todas as contas no cliente DDClassico antes. Ao iniciar, clique na janela da principal.
cd /d "%~dp0"

if not exist ".venv\Scripts\python.exe" (
    echo Rode o rodar_bot.bat uma vez antes, para instalar o Python e as dependencias.
    pause
    exit /b 1
)

".venv\Scripts\python.exe" -m src.grupo %*
pause
