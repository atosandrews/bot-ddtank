# Contexto atual — bot-ddtank

Inspeção documental em 2026-10-29. Esta descrição segue o código encontrado; não certifica execução no jogo nem compatibilidade do ambiente.

## Arquitetura

Bot desktop Windows com Python, OpenCV/NumPy, mss, PyAutoGUI e pywin32. Dependências em [requirements.txt](../requirements.txt).

| Área | Fonte | Papel |
| --- | --- | --- |
| Individual | [main.py](../src/main.py), [bot.py](../src/bot.py) | Seleção da janela, reconhecimento e execução de cliques/turnos |
| Grupo | [grupo.py](../src/grupo.py) | Principal/convidadas, convites, Pronto, combate e saída |
| Visão | [vision.py](../src/vision.py), [telas.py](../src/telas.py) | Template matching e primeira tela reconhecida em ordem de prioridade |
| Combate | [batalha.py](../src/batalha.py) | Estágio/vez, personagem/alvos, ângulo e barra de força |
| Drivers | [drivers](../src/drivers/) | Captura da tela ou PrintWindow, foco e ações reais |
| Ajustes | [config.py](../src/config.py) | Limiares, esperas, estratégias e flags de grupo |
| Verificação automática | [verificacao.py](../src/verificacao.py) | Ler, preencher, confirmar e reavaliar o captcha; ver [contratos e validação](VERIFICACAO_OCR.md) |

O adaptador `verificacao_ocr.py` importa o SDK local `ddddocr`, instalado no ambiente do bot. Não há cliente HTTP entre os projetos. Os repositórios Git continuam independentes. Não foi encontrado uso de Supabase, banco, frontend web ou Playwright.

## Comportamento encontrado

Entrada: canal → popup opcional → lobby/Central → Cais → Jogar → sala → Formigueiro/Normal → Início → lembrete opcional → batalha. `SEM_FOCO` tem prioridade; a batalha usa a âncora visual “Força”. Veja a [especificação ilustrada](especificacao/ESPECIFICACAO.md).

Na estratégia individual, FORMIGAS usa ângulo 30/força 10, com força 1 para inimigo colado; PANDORA usa ângulo 30/força 80 e teclas 3/9. São valores existentes, não recomendações de calibração.

As flags atuais `SO_PRINCIPAL_JOGA = True` e `CONVIDAR_SO_NO_ESTAGIO_2 = True` levam ao estágio inicial com a principal, convite das demais para Pandora, Confiar/PASS nas convidadas e remoção depois. Outros caminhos dependem de configuração e precisam de cobertura própria. `PARAR_DEPOIS_DA_INSTANCIA = True` está configurado; o grupo tem laço próprio, então não presumir encerramento idêntico ao individual.

`DriverTela` captura pixels do desktop: outra janela sobre o jogo altera a imagem. `DriverJanela` usa PrintWindow; `DriverConta` combina captura por janela e ação em primeiro plano. O histórico relata congelamento de desenho em janelas totalmente cobertas; captura em segundo plano não garante atualização.

## Evidências e lacunas

- [README original](../README.md): início rápido; fases antigas não refletem integralmente batalha/grupo implementados.
- [NOTAS.md](../NOTAS.md): decisões e relatos de execuções em 2026-10-28. Resultados são históricos; “Próximos passos” contém itens já superados.
- [Screenshots](especificacao/imagens/) e [templates](../img/) apoiam reconhecimento. [logs](../logs/) recebe diagnósticos que podem conter nomes de contas e informações visíveis da tela.
- A preparação inicial não encontrou suíte; a integração posterior adicionou testes pytest do OCR, visão, driver e controlador em `tests/`. Há também scripts de diagnóstico e regressão; veja [TESTES.md](TESTES.md).

Investigações sugeridas pela inspeção e pelo histórico: leitura da força, foco/atualização das janelas do grupo, transições de Pronto e caminhos alternativos de estratégia. Alguns relatos receberam correções posteriores; reproduzir antes de classificar como defeito atual.

Continue pelo [guia de desenvolvimento](DESENVOLVIMENTO.md), [testes](TESTES.md), [skills](../../docs/SKILLS.md) e papéis [desenvolvedor](../../agents/desenvolvedor.md)/[tester](../../agents/tester.md).
