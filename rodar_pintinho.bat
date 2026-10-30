@echo off
rem Duplo clique para rodar o bot do Pintinho Corre Rapido (uma conta, Dificil, em ciclo).
rem Abra o jogo antes. Ao iniciar, clique na janela da conta que vai jogar.
cd /d "%~dp0"

call preparar_ambiente.bat
if errorlevel 1 (
    pause
    exit /b 1
)

".venv\Scripts\python.exe" -m src.pintinho %*
pause
