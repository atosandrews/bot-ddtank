@echo off
rem Duplo clique para rodar o bot. Na primeira vez cria o .venv e instala as dependencias.
cd /d "%~dp0"

if not exist ".venv\Scripts\python.exe" (
    echo [setup] Criando ambiente virtual .venv ...
    py -3 -m venv .venv 2>nul || python -m venv .venv
)
if not exist ".venv\Scripts\python.exe" (
    echo Python nao encontrado. Instale o Python 3 em https://www.python.org/downloads/
    echo e marque "Add python.exe to PATH" na instalacao.
    pause
    exit /b 1
)
if not exist ".venv\.instalado" (
    echo [setup] Instalando dependencias ...
    ".venv\Scripts\python.exe" -m pip install -r requirements.txt && echo ok> ".venv\.instalado"
)

".venv\Scripts\python.exe" -m src.main %*
pause
