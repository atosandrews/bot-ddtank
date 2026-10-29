# Desenvolvimento — bot-ddtank

Leia [CONTEXTO.md](CONTEXTO.md), [AGENTS.md](../AGENTS.md) e [VERIFICACAO_OCR.md](VERIFICACAO_OCR.md). A preparação documental inicial foi seguida de instalação e testes offline da integração, sem iniciar o jogo.

## Ambiente Windows isolado

Execute na raiz `bot-ddtank`, com ambiente exclusivo:

```powershell
py --list
py -3.13 -m venv .venv
.\.venv\Scripts\python.exe --version
.\.venv\Scripts\python.exe -m pip install -r requirements.txt
```

Python 3.13.1 foi usado na instalação e validação offline do bot/OCR; o launcher global padrão observado é 3.15. Não é necessário ativar o ambiente ou alterar a política do PowerShell. Se `.venv` já existe, confira a versão antes de usá-lo; não o substitua automaticamente. Para os testes, instale `requirements-dev.txt`, que inclui pytest e o SDK local.

Os arquivos `rodar_bot.bat`/`rodar_grupo.bat` chamam `preparar_ambiente.bat`, que cria ambientes novos com Python 3.13, instala requisitos e só então permite a execução. O requirements inclui `-e ../ddddocr`; mantenha o projeto irmão nesse caminho. As versões não são fixadas integralmente: registre o ambiente observado, sem prometer reprodução exata.

## Alterar reconhecimento e estratégias

1. Reproduza com screenshot salvo e estado esperado.
2. Recorte identificação/alvos em `img/` sem hover, balões ou animações; guarde a imagem de origem em `docs/especificacao/imagens/`.
3. Ajuste `TELAS` preservando a prioridade dos popups. Cada `Clique` usa centro do template, deslocamento e alvo opcional quando configurados.
4. Inclua o caso esperado em `tools/testar_prints.py`; verifique todos os estados, alvos e sobreposições afetadas.
5. Documente fluxo e evidências. Validação no jogo exige execução observada; o teste com imagem não a substitui.

Em batalha, offsets são relativos à âncora “Força”. `ler`, `ler_numero`, `ler_angulo` e `forca_na_barra` têm funções diferentes. Alterações de regiões/limiares precisam de imagens dos estágios e condições afetados. Estratégias ficam em `config.py`; evite parâmetros duplicados.

## Validar e executar

Comece offline, a partir deste repositório:

```powershell
.\.venv\Scripts\python.exe tools/testar_prints.py
.\.venv\Scripts\python.exe tools/testar_batalha.py docs/especificacao/imagens/22_pandora.webp
```

O segundo comando grava `logs/teste_batalha.png`; sem argumento de imagem, tenta capturar a janela ao vivo. Veja [TESTES.md](TESTES.md) para distinguir scripts offline, captura e ações.

Estes comandos controlam o jogo e pertencem à validação interativa solicitada, com janelas/contas selecionadas:

```powershell
.\.venv\Scripts\python.exe -m src.main "DDClássico"
.\.venv\Scripts\python.exe -m src.grupo
```

O individual recebe o título, usa `config.TITULO_JANELA` se ausente e pode perguntar pela janela. O grupo pede seleção da principal e aceita números de janela como argumentos; confira lista corrente e código antes de informá-los. Não identifique principal pelo tamanho: esse comentário antigo em `config.py` não representa a seleção interativa atual.

Mouse/teclado são compartilhados. Controles incluem verificação de foco, interrupção por Ctrl+C/canto da tela e liberação de teclas no encerramento. O controlador automático de verificação suspende o restante das ações; leia [o fluxo e seus limites](VERIFICACAO_OCR.md) antes de testar ao vivo.

## Entrega e diagnóstico

Arquivos de cada execução em `logs/` (fora do git):
- `execucao_AAAAMMDD.txt`: cópia de tudo que o bot, o grupo e o captcha escrevem no terminal.
- `tiros/AAAAMMDD_HHMMSS_tN_<lado>.jpg` (`SALVAR_PRINT_TIRO`): print anotado de cada tiro. O log tem a linha `tiro turno N:` com posição do personagem, `dx` das formigas, alvo, lado escolhido, lado lido pela agulha e se a seta foi apertada.
- `verificacao/` (`VERIFICACAO_SALVAR_DIAGNOSTICO`): recortes do captcha; ver [VERIFICACAO_OCR.md](VERIFICACAO_OCR.md).

Informe mudança, imagens, comandos, códigos de saída e pendências. Registre resolução, DPI/escala, contas, flags e se houve captura ou ação real. Compare template/screenshot antes de reduzir limiar; confira atualização da imagem e foco antes de alterar estratégia do grupo.

Atualize os guias quando o comportamento mudar. Preserve [NOTAS.md](../NOTAS.md) como histórico e acrescente evidências datadas. Consulte o [catálogo de skills](SKILLS.md).
