@echo off
rem Duplo clique para rodar o bot com varias contas (principal + convidadas).
rem Abra todas as contas no cliente DDClassico antes. Ao iniciar, clique na janela da principal.
cd /d "%~dp0"

call preparar_ambiente.bat
if errorlevel 1 (
    pause
    exit /b 1
)

".venv\Scripts\python.exe" -m src.grupo %*
pause
