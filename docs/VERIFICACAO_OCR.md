# Verificação automática com OCR local

Implementação de 29/10/2026, solicitada após a preparação documental. O comportamento atual é ler, clicar nos números, confirmar e continuar tentando enquanto houver um desafio ativo. Não há limite fixo de tentativas nem fallback automático para preenchimento humano. Ctrl+C e o failsafe do PyAutoGUI continuam disponíveis.

## Preparar e executar

Mantenha `bot-ddtank/` e `ddddocr/` lado a lado. O SDK é instalado de forma editável no ambiente do bot; não usa HTTP, serviço externo, GPU nem as APIs FastAPI.

No PowerShell, dentro de `bot-ddtank`:

```powershell
py -3.13 -m venv .venv
.\.venv\Scripts\python.exe -m pip install -r requirements-dev.txt
.\.venv\Scripts\python.exe -m pip check
.\.venv\Scripts\python.exe -B -m pytest -q tests
```

`preparar_ambiente.bat` cria um ambiente novo com Python 3.13 e instala dependências sem iniciar o jogo. Os lançadores `rodar_bot.bat` e `rodar_grupo.bat` o chamam antes de executar automação. Um marcador novo, `.instalado-ocr-v1`, permite atualizar ambientes preparados antes da integração. Ambientes existentes são preservados; confira a versão antes de reutilizá-los.

Os comandos seguintes **controlam o jogo** e não foram usados na validação offline desta entrega:

```powershell
.\.venv\Scripts\python.exe -m src.main "DDClássico"
.\.venv\Scripts\python.exe -m src.grupo
```

## Componentes e fluxo

- [verificacao_ocr.py](../src/verificacao_ocr.py): duas instâncias CPU de `DdddOcr`, padrão e beta, compartilhadas no processo; PNG original, cinza ampliado 2x e a variante `cores` (separa os dígitos por matiz, lê cada camada e junta da esquerda para a direita; descarta o agrupamento quando falta leitura ou quando uma camada de vários dígitos tem outra camada no meio); restrição numérica e deduplicação dos candidatos. `classification()` utiliza os argumentos do SDK local, sem modificar suas interfaces.
- [verificacao_visao.py](../src/verificacao_visao.py): título, recorte do código/contador, casas preenchidas e localização dos dez números/controles na captura atual.
- [verificacao.py](../src/verificacao.py): revalidação, Resetar, sequência de quatro cliques, Confirmar, espera de resultado e novas tentativas. O controlador público é serializado entre contas.
- [drivers/tela.py](../src/drivers/tela.py): captura em primeiro plano e clique verificado, também herdados por `DriverConta`. O caminho de verificação não usa cliques genéricos no centro nem PostMessage.
- [perfil e templates](../img/verificacao/): derivados por recorte da [imagem de referência](especificacao/imagens/38_codigo_verificacao.png), sem alterar o original.

Os modelos são carregados antes do laço individual/de grupo. Quando aparece a verificação, as demais ações ficam suspensas. O controlador confirma que a janela existe, traz somente seu HWND para frente, recaptura pixels e localiza os controles. Antes de cada clique, confere desafio, origem, teclado, estado das quatro casas, foco, geometria e janela sob o destino.

Entrada parcial é limpa com Resetar; após isso o quadro é recapturado, porque Resetar pode mudar o desafio. Mudança de imagem durante preenchimento descarta a sequência. Apenas candidatos de quatro dígitos ASCII com score mínimo são enviados; zeros iniciais e repetições são preservados. Não há conversão de letras parecidas com números.

Após Confirmar, o controlador aguarda resposta. Se o quadro permanece, o candidato é marcado como sem aceitação observada e outro candidato é considerado. Um candidato já rejeitado não é repetido na mesma imagem; sem candidatos novos, continuam as capturas e inferências até mudar a imagem ou acabar o prazo. Nenhum mecanismo garante gerar novos candidatos para uma imagem estática que os modelos não conseguem ler.

## Prazo, resultados e erros

O prazo usa o contador reconhecido, com teto conservador de 30 segundos por imagem/ciclo. Não há cliques depois desse prazo. Um novo desafio reinicia o ciclo; expiração não é contada como sucesso.

`RESOLVIDO` exige envio de uma confirmação nesse desafio, desaparecimento do quadro e duas capturas consecutivas de uma tela conhecida, dentro do prazo. É evidência da interface, sem acesso à resposta do servidor. `RETOMADO` indica desaparecimento sem confirmação automática nesse desafio, por exemplo intervenção externa. `EXPIRADO` indica fim do prazo sem aceitação observada.

Se o quadro expirado permanece, o controlador aguarda mudança de imagem/contador. Se ele desaparece e o jogo retorna sem aceitação observada, o fluxo encerra com erro, sem anunciar resolução ou continuar o combate. Não é possível repetir um desafio que o jogo já removeu. Modelo/instalação ausente, falha de inferência ou fechamento da janela também interrompem o fluxo com mensagem; os lançadores Python retornam código 1 nesses casos. Os caminhos de encerramento liberam as teclas.

O `DriverJanela` experimental sozinho não oferece esse contrato; a automação usa `DriverTela`/`DriverConta` e exige HWND válido.

## Configuração e diagnóstico

As opções `VERIFICACAO_*` estão em [config.py](../src/config.py): título `0.85`, OCR `0.90`, controles `0.90`, margem entre destinos `0.03`, intervalo de ciclo de 1 s, pausa de clique de 0.25 s e espera de resposta de 2.5 s. O score do SDK é uma heurística agregada, sem garantia de acerto. O limiar ainda exige calibração com corpus rotulado.

O perfil inicial é nativo de **517×314 pixels**, com título de **192×19**. Translação do quadro é suportada; outras escalas, renderizações de botão ou DPI não foram validados e podem impedir a localização. Não adivinhar coordenadas quando o perfil falha.

Diagnóstico estritamente offline, sem importar drivers:

```powershell
.\.venv\Scripts\python.exe -B tools/testar_verificacao.py docs/especificacao/imagens/38_codigo_verificacao.png --sem-ocr
.\.venv\Scripts\python.exe -B tools/testar_verificacao.py docs/especificacao/imagens/38_codigo_verificacao.png --salvar logs/verificacao_offline
```

Saídas: 0 para quadro válido (e candidato elegível quando OCR habilitado), 1 para erro/quadro ausente e 2 para OCR sem candidato elegível. `--salvar` grava código e controles anotados na pasta indicada. Em automação, `VERIFICACAO_SALVAR_DIAGNOSTICO=True` grava recorte e metadados por tentativa em `logs/verificacao/`; o padrão é False. Logs de tentativas incluem conta, candidato, modelo, score e duração.

## Validação realizada e pendências

Ambiente criado com Python 3.13.1, OpenCV 5.0.0.93, ONNX Runtime 1.31.1, NumPy 2.5.4, Pillow 13.0.0 e pytest 9.2.0. `pip check` passou. O SDK local foi instalado no ambiente do bot, preservando os modelos e o código funcional do projeto OCR.

Os **48 testes** em `tests/test_verificacao_*.py` passaram. Usam OCR, relógio e drivers simulados; os testes visuais usam a imagem existente e variantes sintéticas. Cobrem mapa das teclas, deslocamento, ambiguidade, entrada parcial, candidato rejeitado, mais de cem tentativas, novo desafio, perda de foco, geometria, fechamento, expiração, isolamento entre contas, integração dos dois laços e Ctrl+C. A regressão das 15 telas existentes passou.

**Limitação observada com modelos reais (estado inicial):** o quadro, contador 30 e os dez números foram reconhecidos na imagem existente, mas nenhum dos quatro resultados modelo/variante produziu quatro dígitos elegíveis. O diagnóstico devolveu lista vazia de candidatos e saída 2. Portanto, esta entrega implementa e testa o controlador, mas **não comprova resolução automática desse captcha ou acurácia no jogo**.

Como smoke separado de contrato, uma imagem sintética de texto limpo `0011` foi processada pelo adaptador com os modelos reais: o modelo padrão retornou `0011` com score aproximado de 0.9965. Isso confirma preservação de zeros e execução do SDK; não substitui a validação com ruído/distorção do captcha real.

### Variante por cores (2026-10-29, posterior)

Investigação offline da imagem 38: o código é `6723` (6 roxo sobreposto ao 7 vermelho, 2 vermelho, 3 azul, com linhas claras de ruído). Lido inteiro, o recorte vira dois dígitos (`63`/`23`); só removendo o ruído, três (`723`). A causa é a sobreposição de dígitos de cores diferentes. Lendo cada camada de cor separadamente, os dois modelos reais retornam `6723`.

Implementado em `verificacao_ocr.py` como variante adicional (as anteriores continuam). Resultados com modelos reais nesta imagem: único candidato `6723` (beta 0,989; padrão 0,954), `testar_verificacao.py` com saída 0, cerca de 0,28 s por leitura. Agrupamentos com roxo e azul juntos produziam `7263` (errado); a regra de camada intercalada os descarta e há teste para isso. Suíte: **53 testes** passaram, regressão das 15 telas passou.

Validado em **uma** imagem real; limiares de cor (saturação ≥ 80, brilho ≤ 200, área ≥ 40 px) e número de cores (2 a 4) não foram calibrados em outras. Não comprova acurácia no jogo.

Ainda faltam imagens rotuladas adicionais e validação em sessão real dos estados aceito, rejeitado e expirado; também do hover, aparência das casas preenchidas e variação dos botões. Não houve captura ao vivo, clique real ou execução do jogo nesta entrega.
