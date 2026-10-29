@echo off
rem Prepara dependencias; nao inicia o jogo nem controla mouse/teclado.
cd /d "%~dp0"
if not exist ".venv\Scripts\python.exe" (
    py -3.13 -m venv .venv
)
if not exist ".venv\Scripts\python.exe" (
    echo Instale Python 3.13 para criar o ambiente .venv.
    exit /b 1
)
if not exist "..\ddddocr\pyproject.toml" (
    echo Projeto irmao ..\ddddocr nao encontrado. Mantenha os dois diretorios no contexto.
    exit /b 1
)
if not exist ".venv\.instalado-ocr-v1" (
    echo [setup] Instalando bot e SDK OCR local ...
    ".venv\Scripts\python.exe" -m pip install -r requirements.txt
    if errorlevel 1 exit /b 1
    echo ok> ".venv\.instalado-ocr-v1"
)
exit /b 0
