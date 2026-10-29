# Testes do bot DDTank

Este guia separa os scripts existentes dos cenários de regressão propostos. Leia também [CONTEXTO](CONTEXTO.md), [DESENVOLVIMENTO](DESENVOLVIMENTO.md), as [instruções do projeto](../AGENTS.md), o [papel do tester](../../agents/tester.md) e o [catálogo de skills](../../docs/SKILLS.md).

## Estado verificado em 2026-10-29

Atualização posterior à preparação inicial: ambiente `.venv` do bot instalado, suíte de integração OCR e regressão das 15 telas executadas. Leia [VERIFICACAO_OCR.md](VERIFICACAO_OCR.md) para comandos, resultados e a limitação de reconhecimento observada na imagem real. Os quatro itens abaixo registram o estado **anterior**, durante a preparação documental.

- Inspeção do código, scripts, fixtures e metadados de dependências. Nenhum jogo, captura, troca de foco, script de testes ou instalação foi executado nesta entrega.
- Não foi encontrada suíte pytest/unittest nem configuração de CI. Os arquivos `tools/testar_*.py` são scripts independentes; somente `testar_prints.py` compara um conjunto de expectativas e termina com código de falha.
- O Python padrão é 3.15.1. Também há Python 3.13.1, com NumPy 2.5.1 e Pillow 12.2.0. Os metadados consultados não indicam OpenCV, mss, PyAutoGUI ou pywin32 nos dois interpretadores. O comportamento em runtime continua **não validado**.
- Não existe integração implementada com o projeto irmão ddddocr. Os cenários abaixo usam a visão OpenCV e os drivers atuais do bot.

## Preparação e comandos existentes

Use um ambiente isolado conforme [DESENVOLVIMENTO](DESENVOLVIMENTO.md). Execute os comandos abaixo em `ddtank-context/bot-ddtank`, com as dependências do projeto já instaladas. Os exemplos usam explicitamente `.venv\Scripts\python.exe`; não substitua pelo Python global sem conferir o ambiente.

| Comando PowerShell | Categoria e efeito | Como avaliar |
| --- | --- | --- |
| `.\.venv\Scripts\python.exe tools/testar_prints.py` | Offline: lê imagens e templates; imprime confiança e resultado; não usa o jogo | As 15 expectativas devem passar. Código de saída 0 significa sucesso, 1 significa falha; erro de leitura/importação também precisa ser investigado |
| `.\.venv\Scripts\python.exe tools/testar_batalha.py docs/especificacao/imagens/20_batalha_minha_vez.webp` | Offline: lê screenshot; grava/substitui `logs/teste_batalha.png`; não aperta teclas | Conferir leitura de estágio, turno, personagem, inimigos, ângulo e força com a imagem; o script não contém asserções desses valores |
| `.\.venv\Scripts\python.exe tools/listar_janelas.py` | Consulta interativa: enumera janelas Windows; não é teste offline | Conferir título e identificador da janela pretendida |
| `.\.venv\Scripts\python.exe tools/diagnosticar.py "TITULO DA JANELA"` | Captura interativa: pode restaurar/trazer a janela para frente; grava `logs/diagnostico.png` | Conferir tamanho, posições, confiança e tela identificada |
| `.\.venv\Scripts\python.exe tools/testar_template.py img/canal_2.png "TITULO DA JANELA"` | Captura interativa: pode mudar foco; grava `logs/teste_template.png` | Conferir posição encontrada e marcação; ausência não constitui asserção automatizada |
| `.\.venv\Scripts\python.exe tools/testar_printwindow.py` | Captura interativa por hwnd: grava `logs/printwindow_<hwnd>.png` | Conferir brilho, imagem preta, dimensão e tela identificada em cada janela |
| `.\.venv\Scripts\python.exe tools/testar_batalha.py` | Captura interativa: sem arquivo, usa a janela real e pode mudar foco; grava `logs/teste_batalha.png` | Mesma análise visual da variante offline |
| `.\.venv\Scripts\python.exe -m src.main` | **Ação real:** executa o bot, com mouse e teclado | Somente sessão manual de validação do fluxo e estratégias |
| `.\.venv\Scripts\python.exe -m src.grupo` | **Ação real:** coordena contas, foco, convites, sala e batalha | Somente sessão manual com todas as contas identificadas |

`rodar_bot.bat` também cria ambiente e instala dependências na primeira execução; portanto não é comando de inspeção. `rodar_grupo.bat` inicia automação real. Para interromper uma sessão, use Ctrl+C; o README também informa o failsafe do PyAutoGUI ao mover o mouse para um canto da tela.

## Fixtures e expectativas

As imagens estão em [especificacao/imagens](especificacao/imagens/), com 38 arquivos na inspeção. A [especificação](especificacao/ESPECIFICACAO.md) fornece o fluxo; `tools/testar_prints.py::ESPERADO` é a fonte das 15 expectativas automatizadas atuais. Os templates são os arquivos de `img/`, incluindo `img/digitos/`; não trate screenshots inteiros como templates.

| Fixtures existentes | Uso atual ou proposto |
| --- | --- |
| `01_tela_selecao_canal.png`, `03_tela_aluno_recomendado.png`, `05_tela_lobby_principal.png`, `07_tela_cais_expedicao.png` | Regressão atual de canal, popup, lobby e cais |
| `09_tela_sala.webp`, `16_tela_sala_janela_real.png`, `17_tela_configuracao_sala.png`, `18_configuracao_sala_janela_real.png` | Regressão atual de sala e seleção de instância |
| `14_botoes_convide_troque_inicio.png`, `15_popup_lembrete.png` | Recortes parciais: validam reconhecimento e alvos previstos, sem representar toda a tela |
| `19_batalha_inicio_janela_real.png`, `20_batalha_minha_vez.webp`, `22_pandora.webp`, `24_formigas_minha_vez_janela_real.png` | Regressão atual da tela BATALHA; detalhe de turno, estágio e força exige avaliação adicional |
| `25_clique_no_jogo_janela_real.png` | Regressão atual de SEM_FOCO |
| `32_sala_grupo_prontos.webp`, `33_batalha_grupo_duplas.webp`, `34_convite_com_instancia.webp`, `35_sala_convidada_cancele.webp` | Material para os roteiros de grupo propostos; fora do mapa atual de 15 expectativas |

Para registrar uma nova fixture, anote origem, dimensão, escala/DPI, estado esperado e o elemento que comprova a expectativa. Preserve os originais e mantenha resultados anotados em `logs/`. Uma imagem de batalha não prova acerto de tiro ou tempo de resposta: esses dependem de sequência de frames e/ou sessão manual.

## Matriz de cenários propostos

P0 bloqueia uso do fluxo alterado; P1 cobre comportamento principal; P2 cobre robustez e diagnóstico. Exceto pelas 15 expectativas de reconhecimento, esta matriz é um **roteiro proposto**, não uma suíte entregue ou executada.

| Prioridade | Cenário | Camada e critério de aceitação |
| --- | --- | --- |
| P0 | Verificação automática aparece, é rejeitada, muda ou expira | Suíte `tests/test_verificacao_*.py` com drivers/OCR simulados; validar ao vivo separadamente. Suspender combate, conferir os cliques, tentar novo candidato, distinguir aceitação e expiração |
| P0 | Janela perde foco ou outra conta ocupa o primeiro plano | Driver simulado + manual: nenhuma ação deve atingir janela alheia; registrar falha ao obter foco |
| P0 | Exceção/interrupção durante carregamento de força | Driver simulado: verificar liberação de Espaço e encerramento; confirmar manualmente quando o caminho de input for alterado |
| P1 | Telas conhecidas e alvos obrigatórios | Executar `testar_prints.py`; 15 resultados corretos e alvos acima do limiar configurado |
| P1 | Popup ou seleção de instância sobre a sala | Offline: primeira tela identificada respeita a prioridade de `TELAS`; não escolher elementos da tela de fundo |
| P1 | FORMIGAS/PANDORA e vez ativa/inativa | Offline com imagens rotuladas: estágio e turno concordam com a evidência visual; nenhuma decisão de tiro fora da vez |
| P1 | Ângulo, alvos, aliado, inimigo colado e estratégias por modo | Offline + driver simulado: comparar leitura e decisão às configurações atuais de solo/grupo; não reutilizar expectativa de um modo no outro |
| P1 | Distância e interpolação de força | Testes propostos de função pura: valores inteiros/intermediários, limites 1–20 e cada ângulo suportado, comparados às tabelas |
| P1 | Fluxo canal → sala → batalha → resultado e grupo | Manual: seguir a especificação, verificar principal/convidadas e registrar as transições observadas |
| P1 | Primeiro tiro da Pandora ou leitura incorreta da barra | Sequência de frames/manual: comparar força pedida, leitura ao soltar e marcador; problemas históricos de `NOTAS.md` não contam como resolvidos sem evidência nova |
| P2 | Template ausente, maior que o frame, imagem ilegível ou janela coberta | Offline/diagnóstico: registrar exceção ou ausência coerente; separar erro de fixture de falha de reconhecimento; verificar PrintWindow preto |
| P2 | Resolução, DPI, brilho de hover e limites de confiança | Offline/manual: registrar condições e falsos positivos/negativos; comparar templates recortados sem o mouse sobre o botão |

## Evidências de cada execução

Registre data, commit, interpretador/versões, comando completo, cwd, fixture, parâmetros alterados, resultado esperado/obtido e código de saída. Em sessão real, inclua título/dimensão/DPI da janela, modo solo/grupo, horário da ocorrência e screenshots/logs relevantes. Use **aprovado**, **falhou** ou **não executado**, acrescentando o motivo quando houver bloqueio de ambiente. Para inspeção, use **conferido estaticamente**, sem aprovação funcional.

A preparação inicial realizou somente inspeção/metadados. A integração posterior executou testes simulados, regressão e inferência offline conforme [o registro atualizado](VERIFICACAO_OCR.md); jogo, captura ao vivo e aceitação real continuam não executados.
