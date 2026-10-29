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

- [verificacao_ocr.py](../src/verificacao_ocr.py): duas instâncias CPU de `DdddOcr`, padrão e beta, compartilhadas no processo; PNG original, cinza ampliado 2x, `mascara` (só os pixels saturados e escuros, sem as linhas claras) e a variante `cores` (separa os dígitos por matiz, lê cada camada e junta da esquerda para a direita; descarta o agrupamento quando falta leitura ou quando uma camada de vários dígitos tem pedaço de outra camada no meio); restrição numérica e decisão por consenso (`decidir`, abaixo), que devolve no máximo um candidato. `classification()` utiliza os argumentos do SDK local, sem modificar suas interfaces.
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

### Consenso entre leituras e correção da ordem em `cores` (2026-10-29, posterior)

Motivo: na avaliação sintética do tester (`tools/avaliar_ocr.py`, semente 1) o bot enviaria código errado em 1–8% dos casos, quase sempre com score 0,92–0,999; subir o limiar não separa erro de acerto.

- **Bug de ordem em `cores`** (6972→6792, 9254→9524): a regra de camada intercalada comparava só o centroide do *maior* pedaço da outra camada com o intervalo da camada de vários dígitos. Com cores A B A B, o maior pedaço de cada cor pode cair fora do intervalo da outra. Agora qualquer pedaço (≥ `AREA_MIN_CAMADA`) de outra camada dentro do intervalo descarta o agrupamento (`_intercaladas`).
- **Decisão por consenso** (`decidir`): (1) unanimidade — se as leituras de quatro dígitos divergem, com qualquer score, não há candidato; (2) o código precisa de duas fontes independentes com score ≥ limiar: os dois modelos, ou duas famílias de pré-processamento (`bruta` = original/cinza_2x, `mascara`, `cores`). original e cinza_2x do mesmo modelo quase sempre erram juntos (ex.: 1→7), por isso contam como uma fonte. O resultado é no máximo um candidato; depois de rejeitado, o bot espera nova imagem em vez de tentar a segunda opção.
- Nova variante `mascara`: mais uma fonte independente (+2 inferências).

Regras escolhidas com a semente 1 (cache das leituras de 300 por nível) e medidas numa semente **não usada no ajuste**: `--n 300 --semente 2`, limiar 0,90, mesma máquina. "Antes" = código do commit 4e5a9de na mesma semente.

| Nível (semente 2) | ERRADO antes → depois | certo antes → depois | sem candidato antes → depois | tempo médio antes → depois |
| --- | --- | --- | --- | --- |
| fácil | 1,0% → 0,0% | 99,0% → 98,3% | 0,0% → 1,7% | 0,53 → 0,59 s |
| médio | 5,0% → 0,7% | 92,7% → 76,0% | 2,3% → 23,3% | 0,49 → 0,52 s |
| difícil | 12,0% → 2,0% | 77,0% → 54,3% | 11,0% → 43,7% | 0,47 → 0,58 s |
| adversarial | 6,3% → 0,7% | 86,3% → 65,3% | 7,3% → 34,0% | 0,41 → 0,52 s |
| imagem 38 (real) | 0 → 0 | 6723 → 6723 | — | 0,29 → 0,42 s |

Na semente 1 (usada no ajuste) a mesma regra dá ERRADO 0,0 / 0,3 / 0,7 / 0,3%. Arquivos: `logs/avaliacao_ocr_base_semente2.json` e `logs/avaliacao_ocr_dev.json`. O avaliador foi depois atualizado: o top-1 "geral" é o primeiro candidato de `reconhecer()`, ou seja, o que o bot envia; `--modulo-ocr` compara com outra versão do adaptador.

Troca explícita: cobertura menor em troca de bem menos envios errados. Sem candidato, o bot recaptura até a imagem mudar ou o prazo acabar; numa imagem estática a leitura se repete. Uma alternativa medida na semente 1 — vetar só quando duas leituras divergentes têm cada uma os dois modelos — dava ERRADO 0,83% e certo 81,6% (contra 0,33% e 73,8%), sem separação por número de votos entre erros evitados e acertos perdidos. Restam erros em que todas as leituras concordam no código errado (ex.: 0231 lido 0237 pelas seis fontes). Tudo isso é sintético; não comprova taxa no jogo.

Ainda faltam imagens rotuladas adicionais e validação em sessão real dos estados aceito, rejeitado e expirado; também do hover, aparência das casas preenchidas e variação dos botões. Não houve captura ao vivo, clique real ou execução do jogo nesta entrega.

**Validação independente (tester, semente 3, n=300):** ERRADO antes → depois: fácil 1,0 → 0,0%; médio 5,7 → 0,3%; difícil 9,7 → 1,3%; adversarial 4,3 → 0,7%. Certo: 99,0 → 97,0; 92,7 → 76,0; 81,0 → 53,0; 87,3 → 67,3%. Imagem 38: 6723 nos dois.

**Correção seguinte em `_intercaladas`:** quando dígitos da mesma cor se ligam por cima de outra camada, viram um pedaço só e o intervalo entre centroides some. Agora cada pedaço guarda a extensão x, e uma camada de vários dígitos com pedaço único usa a metade central da largura. Coberto por teste. Conferência final na semente 4 (n=150, não usada em nenhum ajuste): ERRADO 0,0 / 0,0 / 0,67 / 0,0%, certo 98,0 / 79,3 / 61,3 / 70,7%, imagem 38 certa.

**Impacto de "sem candidato" no fluxo:** o bot recaptura a mesma imagem; se o quadro some ao fim do prazo, `tratar()` levanta `ErroInfraestrutura` e o bot para. `Resetar` só é clicado para limpar casas preenchidas; não se sabe se troca o código. Validar isso no jogo é o caminho para recuperar cobertura sem enviar código errado.

Reproduzir: `tools/avaliar_ocr.py --n 300 --semente 3 --json logs/avaliacao.json` (amostras: `tools/gerar_captchas.py --n 8 --semente 1 --ampliar 3 --saida logs/captchas_amostra`).
