@echo off
rem Duplo clique para rodar o bot com OCR local.
cd /d "%~dp0"

call preparar_ambiente.bat
if errorlevel 1 (
    pause
    exit /b 1
)

".venv\Scripts\python.exe" -m src.main %*
pause
