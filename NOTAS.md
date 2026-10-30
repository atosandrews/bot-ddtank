# NOTAS do projeto (contexto para retomar em qualquer conversa)

> Atualize este arquivo sempre que avançar. Cole-o no início de uma conversa nova
> com o Claude (junto dos arquivos relevantes) para continuar de onde parou.

## Objetivo
Bot de estudo para automatizar instâncias (dungeons) no **DDTank Clássico**, usando
visão computacional + simulação de input, sem mexer no cliente do jogo.
Meta futura: rodar 4 ou mais bots ao mesmo tempo, sem precisar da tela aberta.

## Meu perfil
- Nível de programação: básico, iniciando agora.
- Linguagem: Python.
- Jogo: **cliente instalado no PC** (Windows), não navegador.
- Começo: quero ver a automação agindo na tela (modo visual).
- Uso: estudo, em servidor privado ou conta descartável (automação pode violar termos e dar ban).

## Arquitetura (4 camadas)
1. **Captura**: pegar frames da janela (`mss`, depois `PrintWindow` por hwnd).
2. **Percepção**: identificar a tela atual e elementos (`opencv` template matching).
3. **Decisão**: máquina de estados (`bot.py`).
4. **Ação**: cliques e teclas (`pyautogui`, depois `PostMessage`).

Regra de ouro: a lógica do bot **não sabe** como a tela é capturada. Tudo passa por um
driver com a interface `capturar()` e `clicar(x, y)`. Trocar de driver não muda o bot.

## Fluxo de uma instância
```
LOBBY -> escolher dungeon -> SALA -> iniciar -> BATALHA -> LOOT -> LOBBY
```
Em cada turno da batalha: detectar vez -> localizar posições -> calcular ângulo e força
-> ajustar -> segurar espaço até a força certa -> disparar.

## Fases
- [x] **Fase 1**: captura da tela/janela + mouse real (`src/drivers/tela.py`).
- [ ] **Fase 2**: captura e clique em segundo plano por hwnd (`src/drivers/janela.py`, experimental).
      Limite: janela minimizada não renderiza; alguns clientes ignoram PostMessage.
- [ ] **Fase 3**: várias instâncias. Um processo por janela (`multiprocessing`).
      Opções: várias janelas com hwnd próprio, desktops virtuais, máquinas virtuais (mais isolado).
- [ ] **Fase 4**: (avançado) sem cliente, falando direto com o servidor. Exige engenharia
      reversa do protocolo e é onde há mais risco de detecção.

## Mira (parte difícil)
- Opção A (começar por aqui): **tabela de calibração** (ângulo e força que acertam em várias
  distâncias, com interpolação).
- Opção B: **modelo físico** da trajetória (gravidade, vento, força), com parâmetros
  descobertos por experimento.
- Detecção de posições: `matchTemplate` no início; depois segmentação por cor ou YOLO.

## Estado atual (v2 — até o início da batalha, 2026-10-28)
- Especificação em `docs/especificacao/ESPECIFICACAO.md` (com os prints do jogo), estados 1 a 8.
- O bot **reconhece a tela atual** a cada ciclo (em vez de seguir uma ordem fixa) e
  executa os cliques dela, em ordem, esperando cada botão aparecer (até `ESPERA_ALVO` s).
  Telas e ações ficam em `src/telas.py` (a ordem da lista importa: popups primeiro):
  | Tela | Reconhece por | Clica em |
  |---|---|---|
  | LEMBRETE (popup, opcional) | título "Lembrete" | OK |
  | ALUNO_RECOMENDADO (popup, opcional) | título "aluno recomendado" | botão X |
  | SELECAO_INSTANCIA | título "Configuração da sala" | Formigueiro -> Normal -> ok |
  | SALA_SEM_INSTANCIA | quadro "Selecione a Instância" | o próprio quadro |
  | SALA_PRONTA | ícone "Troque" | Início (70 px à direita do Troque) -> OK do Lembrete se aparecer; o bot para |
  | CAIS_EXPEDICAO | título "Cais de expedição" | Jogar |
  | SELECAO_CANAL | "2 Canal 2" | Canal 2 |
  | LOBBY | nome "Central" | prédio Central (20 px abaixo do nome) |
- Templates recortados dos prints em `img/`. Os recortes do Canal 2 e do Central são menores
  que os da especificação, porque a barra de lotação e o cenário animado mudam.
  O botão Início é animado (confiança 0.4–0.8), por isso o bot mira pelo Troque.
- **Fluxo completo testado no jogo em 2026-10-28** (13:07): da seleção de canal até a batalha
  do Formigueiro (Normal) começar, em ~42 s, sem intervenção.
  - 1ª tentativa falhou: o recorte do Formigueiro tinha o brilho de mouse em cima (0.69 no jogo)
    e a janela não era reconhecida; como o Troque aparece atrás dela, o bot clicou em Início.
    Correção: reconhecer pelo título "Configuração da sala" e recortar os botões do print real.
  - Depois do Formigueiro as dificuldades viram Fácil / Normal e o Normal muda de lugar.
  - Lição: **recortar templates de prints sem o mouse em cima** (o brilho muda o botão).
- Tela de batalha (print em `logs/` da execução): mostra "Força Vento", ângulo (21), barra de
  força, Energia/Vida, minimapa e "Pressione Espaço com força total e realize um ataque".
- Ajustes (limiar, esperas, título da janela) em `src/config.py`.
- v1 testada nos prints e **no jogo** em 2026-10-28:
  - Janela do cliente: `DDClássico` (já em `src/config.py`). Maximizada no 2º monitor (1920x1032);
    o jogo fica em tamanho original no centro, então os templates batem com confiança 1.00.
  - Fluxo completo em ~18 s: Canal 2 -> fecha popup -> Central -> Cais -> Jogar.
  - **"Jogar" cria uma sala** (ex.: "Sala 1574", eu como Dono, tela "Selecione a instância").
    É o próximo estado a especificar.
  - O popup demora um instante para abrir: no 1º ciclo o X ainda não aparece, mas no seguinte aparece.
  - Na 1ª execução o jogo voltou ao lobby depois do Jogar (causa não confirmada).
  - `tools/diagnosticar.py` tira um print e mostra o que o bot reconhece, sem clicar.
  - `SALVAR_PRINTS = True` salva prints antes de cada clique e depois do último, em `logs/`.
- A moldura do cliente mostra **tabelas de ângulo x distância x força** (Âng. 20/30/50/65):
  ótimo ponto de partida para a tabela de calibração da mira.

## Batalha v1 (força e skills, 2026-10-28) — `src/batalha.py`
- Tela BATALHA reconhecida pelo rótulo "Força"; tudo no HUD é medido a partir dele.
- Na minha vez (PASS na tela): acha eu (barra de vida azul) e inimigos (barras vermelhas),
  pega o mais perto, distância = pixels ÷ `PX_POR_DISTANCIA` (100), força = tabela do `ANGULO`,
  vira para o alvo com a seta, aperta `SKILLS`, segura Espaço e solta quando a barra chega na força.
- Força medida comparando a barra com ela vazia (não depende da cor). Depois do tiro, o log
  mostra a força pedida e a do marcador vermelho, para calibrar.
- `python tools/testar_batalha.py [print]` mostra a leitura sem apertar nada (marca em logs/).
- Testado só em prints e simulação; **ainda não atirou no jogo**.
- Regra do vento (usuário): a cada 0.5 de vento o ângulo sobe/desce um pouco (falta o valor e o lado).
- 1º teste no jogo (14:10): passou 8 turnos com "não achei a mim". Causas: formiga em cima do
  personagem cobre a barra azul, e a janela do Claude por cima do jogo (o print é da tela).
  Correção: aceita pedaços da barra azul e usa o triângulo azul da vez (~106 px acima) como reserva.
- **Estratégia do Formigueiro (usuário):** as formigas sempre chegam perto, então ângulo 30 e
  força 10 (`FORCA_FIXA`), virando para a formiga mais perto. Sem achar alvo, alterna os lados.
- **Ângulo automático:** lido pela agulha vermelha do relógio (agulha da esquerda; erro ~2°),
  ajustado com ↑/↓ até `ANGULO`. Validado em prints de 15, 21 e 50; ainda não testado ao vivo.
  Às vezes aparece uma 2ª agulha (direita) que não bate com o número (inclinação do chão?).
- `tools/capturar_minha_vez.py` espera a minha vez e salva o print, sem apertar nada.
- **2º teste no jogo (14:28): estágio 1 funcionou sozinho.** 6 turnos, ângulo lido 29–31,
  virando para a formiga certa, força 10. Problemas encontrados e corrigidos:
  - 1º tiro saiu com força 100: a animação "Sua vez!" passou sobre a barra. Agora a força só
    conta a faixa contínua desde o 0.
  - **Nunca deixar outra janela por cima do jogo** (o print é da tela). Pior: teclas iriam
    para a janela da frente. Agora o driver confere o foco antes de cada tecla/clique e,
    se não conseguir trazer o jogo para frente, não aperta nada.
  - O marcador vermelho do último tiro lia sempre 1: removido do log por enquanto.
- **Estratégia por estágio** (`config.ESTRATEGIAS`), estágio lido no quadro de missão
  ("Derrote Pandora" = template `img/missao_pandora.png`; senão é FORMIGAS):
  - FORMIGAS: ângulo 30, força 10, vira para a formiga mais perto.
  - PANDORA (rainha, à direita): ângulo 30, força 80, itens **3 (+2)** e **9 (dano max)** a cada
    turno. Se o chão não deixa o ângulo 30 (agulha trava ao apertar ↑/↓), anda para trás (←)
    `PASSO_ANDAR` s e tenta de novo, até `MAX_PASSOS`. Sem energia, o jogo ignora o item.
  - Testado em prints e simulação; ainda não rodou no estágio 2 de verdade.
- **Agulha recalibrada:** pivô (-75, -36) ajustado com 6 prints de ângulo conhecido; erro ~1°
  nos dois lados (antes a agulha da direita lia ~6° a menos e parecia "inclinação do chão").
- **3º teste no jogo (15:03), fluxo completo:** entrada OK, estágio 1 OK (8 turnos). Entre os
  estágios o bot viu a sala e clicou em Início (não atrapalhou). No estágio 2 não jogou: o jogo
  mostrou **"Clique no jogo, para continuar jogando"** (perdeu o foco do teclado quando o usuário
  clicou no Claude), escureceu a tela (PASS não era visto) e ignorava as teclas.
  Correção: tela SEM_FOCO (`img/clique_no_jogo.png`) clica no aviso. Trazer a janela para frente
  NÃO basta; o jogo precisa de um clique dentro dele.
  - Falso "ângulo travou" no estágio 1: lia a agulha rápido demais. Agora toque de 0.06 s,
    espera 0.12 s e só considera travado após 10 toques sem andar 1.5°.
- **4º teste (15:13):** entrada e estágio 1 OK de novo. No estágio 2 o bot não recebia a vez:
  o PASS é semitransparente e na caverna escura batia só 0.80 (limiar era 0.85; fora da vez
  ~0.33). Agora `LIMIAR_PASS = 0.65`. `tools/gravar_prints.py` grava prints a cada N s sem
  apertar nada (foi assim que achei). No print da Pandora o relógio mostrava 11 e a agulha
  leu 7.6: a leitura pode errar mais em ângulos baixos.
- **5º teste (15:24): ciclo completo sozinho.** Estágio 1 (7 turnos, ângulo 29–31 sempre),
  estágio 2 (turnos 8–13: itens 3 e 9, força 80), fim da batalha às 15:30:35 e o bot já
  começou a próxima instância (Início → Formigueiro → Normal → ok → Início → OK).
  Pendente no estágio 2: em 3 de 6 turnos o ângulo "travou" (35/33, 49/48, 41/38) e andar
  6 passos para trás não resolveu, então atirou com o ângulo que deu. Agora salva
  `logs/*_angulo_travou.png` no 1º travamento do turno para investigar.
- **Usuário confirmou que deu certo**, mas viu o bot andar com o ângulo já em 30: a agulha
  lia 35 (o relógio é semitransparente e o fundo marrom da caverna confunde o vermelho), ele
  andou e o chão mudou o ângulo para 49, errando vários tiros.
  **Regra do usuário:** se o relógio mostra 30, não anda nem mexe: usa os itens e força 80.
  Se não mostra 30, anda até ficar 30 e depois não se mexe mais.
  Correção: o bot agora lê o **número do relógio** (`ler_numero`, modelos em `img/digitos/`,
  comparando só o formato porque a cor muda). Conhecidos: 0–5 (o "1" tem 3 variações).
  Só aceita quando lê o número exato; com dígito desconhecido (6–9) usa a dezena para a
  direção; sem número, a agulha. "Travou" = número não muda em 4 toques. Nos prints ao vivo
  todos os números saíram exatos (inclusive todos os "30").
  Também só aperta a seta de virar quando precisa (1º turno do estágio ou depois de andar),
  porque a batidinha faz o personagem andar um pouco.
- **6º teste (15:48): instância completa e vitória, sozinho, em ~4 min.** Sala → Formigueiro
  Normal → estágio 1 (10 turnos, ângulo 30 exato em todos, sem andar) → estágio 2 (Pandora
  morta em 2 turnos, ângulo 30 exato, itens 3 e 9, força 80) → parou sozinho
  (`PARAR_DEPOIS_DA_INSTANCIA`). Chat: "Você ganhou Pedra de Tigre Branco nível 2" e
  "Pedra de Dragão Verde nível 2".
  Pendente: o 1º tiro de cada estágio da Pandora sai com força ~91 em vez de 80 (aconteceu nos
  dois testes); os outros tiros saem 79–80.
- **Formiga colada (usuário):** no estágio 1 a última formiga ficou exatamente em cima do
  personagem; a barra vermelha dela some atrás da minha barra azul, o bot "não achava
  inimigos" e atirava força 10 para os lados eternamente (usuário matou com força 1).
  Agora, só nas FORMIGAS: inimigo a até `colado_px` (25 px) ou eu visível sem nenhum
  inimigo = colado → `forca_colado` (1), alternando os lados. Testado nos 3 prints reais.
- **Batidinha para virar (usuário):** no 1º turno da Pandora o ângulo estava 15 e o bot
  "andou" antes de subir: era a seta → de virar, apertada por garantia com o personagem já
  virado. Agora `lado_virado` lê o lado pela agulha vermelha forte do relógio (aponta para
  onde ele olha; confere em todos os prints reais) e só vira se estiver do lado errado.
- **7º teste (16:03), commit b7172ab: vitória de novo, sozinho, em ~3,5 min.** Estágio 1 em
  6 turnos, Pandora em 2 turnos; chat "Você ganhou Pedra de Tigre Branco nível 2 / nível 1".
  Desta vez nenhuma formiga colou, então a força 1 ainda não foi testada ao vivo.
  O 1º tiro da Pandora saiu de novo com força 91 (3ª vez seguida); o 2º, 79.
- Ferramenta de Captura do Windows travou com 32 cópias abertas (não era o bot); fechadas.
  O bot agora solta Espaço e setas sempre que encerra.
- SALA_PRONTA não é mais o fim: depois do Início o bot segue para a batalha. O fim da batalha
  e o loot ainda não são reconhecidos (o bot fica esperando e salva print em logs/).

## Fase 3: 4 contas juntas no Formigueiro (requisitos do usuário, 2026-10-28)
- 4 janelas do cliente DDClássico no mesmo PC, uma conta em cada.
- **Teste técnico OK:** `tools/testar_printwindow.py` tira print da janela do jogo mesmo coberta
  (PrintWindow), e o bot reconhece a tela. Janela inativa mostra "Clique no jogo" escurecido.
  Atenção: o filtro de título também pega aba do Opera com "DDClássico" no título; usar título
  exato "DDClássico".
- Plano: um bot só vigia as 4 janelas (PrintWindow) e traz para frente só a que precisa agir
  (teclado/mouse reais), clicando no aviso de foco quando precisar.
- **Entrar na sala:** a principal clica em **Convide** e chama as 3 contas, que aceitam.
  Só na 1ª vez: depois elas continuam na sala entre as instâncias.
- **Ciclo:** as 3 clicam em **Pronto**; a principal espera as 3 prontas, escolhe
  Formigueiro → Normal e clica em Início. Repete sempre.
- **Estágio 1:** 2 contas atiram sempre para a esquerda e 2 sempre para a direita. As duplas
  nascem perto, em posição aleatória: as 2 mais próximas formam a dupla e uma anda até ficar no
  mesmo pixel da outra (para não se acertarem). Tiro: ângulo 30, força 10, sempre com a
  tecla 9 (todos os buffs de dano). Formiga colada: força 1 (regra continua valendo).
- **Estágio 2:** as 4 contas usam a lógica da Pandora de hoje (ângulo 30, andar só se não
  der, teclas 3 e 9, força 80).
- **v1 do grupo escrita (2026-10-28), ainda não rodou no jogo:** `python -m src.grupo` ou
  `rodar_grupo.bat`. Funciona com 2 a 4 contas.
  - `src/drivers/conta.py`: vê cada janela por PrintWindow (mesmo coberta) e traz para frente
    só para clicar/teclar, clicando antes no "Clique no jogo" se aparecer.
  - Janela sem foco sempre mostra "Clique no jogo" escurecido; para ver a tela o bot ignora o
    aviso (a comparação de imagens não muda com o escurecimento).
  - **Principal:** ao iniciar, o bot pede para clicar na janela dela (as janelas maximizadas
    têm o mesmo tamanho, então "a maior" não serve). Ou passar o número da janela.
  - Convite: Convide → aba Amigos → clica em todos os "Convidar". Só quando a principal está
    na sala e alguma convidada está fora (Cais, lobby ou tela desconhecida); repete a cada 30 s.
  - Convidada: "Convite de" → Confirmar. Na sala: **Pronto, suposto no lugar do Início**
    (Troque + 70 px). Ainda não tenho print da sala vista pela convidada: conferir no teste.
  - Principal só clica em Início quando conta os carimbos "Pronto!!" (só o vermelho deles)
    iguais ao número de convidadas.
  - Batalha: quem tem a vez joga. Em grupo, "eu" = triângulo azul da vez (tenta vários prints,
    é animado); as outras barras azuis são aliados. Estágio 1: dupla = aliado mais próximo
    (até 150 px), anda em passinhos até ficar a até 4 px, dupla da esquerda atira para a
    esquerda; só com 2 contas, cada uma atira para o seu lado. Tecla 9, força 10 (1 colada).
  - **Descoberta (18:55): janela totalmente coberta CONGELA** (0 pixels mudam em 4 s; o
    convite chega mas não aparece). Só a janela visível desenha. Quando o Claude ficou no
    monitor do jogo, as 3 ficaram pretas. Consequências: (1) o convite agora é aceito
    "visitando" cada convidada (traz para frente, espera o convite, Confirmar); (2) para a
    batalha, as janelas precisam ficar num monitor só delas, cada uma com um pedaço visível
    (`organizar_janelas` põe uma em cada canto, 1366x780). Ideia do usuário de entrar pelo
    número da sala não resolve isso (a janela coberta também não vê o Cais).
  - Convite só pelo nome: recortes dos nomes em `img/convidados/`; clica só no Convidar da
    mesma linha (antes, com a aba errada, convidou desconhecidos).
  - **Estado em 2026-10-28 20:12 (usuário parou o bot):** convite, Confirmar e entrada na
    sala funcionam; uma partida começou e a Pandora já foi vencida em grupo uma vez.
    Modo atual: `SO_PRINCIPAL_JOGA` (convidadas no Confiar). Estágio 1: na 1ª vez de cada
    convidada ela anda até ficar em cima da principal (acha a principal pelo nome,
    `img/nome_principal.png`, só o formato das letras azuis), liga o Confiar e passa o turno;
    a principal fica parada e alterna os lados. Problemas abertos:
    - Pronto: o botão da convidada às vezes não vira "Cancele" a tempo e o bot clica de novo,
      desmarcando (o mesmo botão marca e desmarca). Precisa conferir sem reclicar.
    - Principal às vezes perde o 1º turno (barra parada: jogo sem foco) e em 2 partidas
      saiu do combate sozinha (causa desconhecida).
    - Andar até a principal parou no meio uma vez (101 → 51 px).
    - Trocar de janela é frágil porque as janelas do jogo dividem o monitor com outros
      programas (janela coberta congela). Monitor só para o jogo ajudaria muito.
  - **Ciclo atual (2026-10-28 20:30, `CONVIDAR_SO_NO_ESTAGIO_2`):** estágio 1 a principal
    faz sozinha (padrão de uma conta, confirma o Lembrete). Na sala do estágio 2 convida as
    convidadas, que confirmam e dão Pronto; ela inicia. Na Pandora a principal joga primeiro
    (ângulo 30, 3 e 9, força 80) e só depois o bot visita cada convidada: Confiar + PASS.
    Depois da Pandora a dona expulsa as convidadas pelo X do cartão
    (`img/sala_expulsar_x.png`; o 1º X, em cima à esquerda, é o dela) e recomeça.
  - **Teste das 21:40 (2026-10-28): ciclo completo em grupo funcionou.** Estágio 1 sozinha →
    convite (as 2 confirmaram) → Pronto das 2 → "2 de 2: iniciando" → Pandora (principal
    primeiro, depois Confiar + PASS nas 2) → expulsou as 2 pelo X → começou o estágio 1 de
    novo. Usuário parou o bot. Um tiro da Pandora saiu com força 23 (barra mal lida).
  - Otimizações (21:55): ângulo espera o número do relógio mudar depois de cada toque (antes
    lia o número antigo e oscilava 29↔30); batalha em grupo tem caminho rápido (só a janela
    da principal) e o código de verificação é conferido a cada 5 s.
  - **Teste 22:00 (com as otimizações): 2 ciclos completos seguidos, sem erro, ~4 min cada.**
    Estágio 1 sozinha (7–9 turnos) → convite/Pronto → Pandora em 2 turnos (Confiar + PASS
    nas convidadas) → expulsou as 2 → recomeçou. Nenhum "ângulo travou" no log.
  - Velocidade (22:20): ângulo em duas velocidades (uma rajada rápida até 3° do alvo, espera
    o relógio parar, depois toque a toque); 15→30 caiu de ~6 s para ~1,4 s (simulação). Esperas
    com sorteio para parecer gente: itens 0,12–0,22 s, virar 0,08–0,14 s, cliques 0,25–0,45 s,
    transições 0,8–1,3 s, início de batalha 1 s, trocar de janela 0,3–0,5 s.
  - **Mouse/teclado:** o bot usa os de verdade, então disputa com o usuário. Saída proposta:
    rodar jogo + bot numa máquina virtual (ou outro PC).
  - Ensaio sem cliques nas janelas reais: principal na sala, conta2 no Cais → clicaria em
    Convide (1257, 714). Certo.

## Código de verificação (anti-bot do jogo)
- Quadro "Código de verificação", 30 s para digitar 4 dígitos clicando nos números.
- **O bot NÃO resolve** (seria burlar a proteção do servidor). `src/verificacao.py`: ao ver
  o título (`img/codigo_verificacao.png`), para tudo, traz a janela para frente, apita e
  espera a pessoa digitar (até 3 min); quando o quadro some, continua. Vale para o bot de
  uma conta e para o grupo (confere todas as janelas a cada volta).

## Próximos passos
1. Testar a batalha v1 no jogo (ponteiro em `ANGULO`) e calibrar `PX_POR_DISTANCIA` e `FORCA_ANTECIPACAO`.
2. Ler e ajustar o ângulo (dígitos do relógio) e compensar o vento.
3. Reconhecer o fim da batalha e o loot; voltar ao início (loop de instâncias).
3. Só depois: lógica de turno e mira.
4. Só depois: Fase 2 (segundo plano) e Fase 3 (várias instâncias).

## Dicas aprendidas
- Fixe resolução e posição da janela, senão os templates deixam de bater.
- Adicione atrasos aleatórios entre ações.
- Salve screenshots (`logs/`) quando o bot se perder, para depurar.
- Mantenha `.env` fora do Git se um dia guardar credenciais.

## Como rodar
Duplo clique em `rodar_bot.bat` (na 1ª vez cria o `.venv` e instala tudo).
Python 3.12 instalado em 2026-10-28 via winget (só para o usuário).

Pelo terminal:
```bash
.venv\Scripts\activate
python tools/testar_prints.py
python tools/listar_janelas.py
python -m src.main "TITULO DA JANELA"
```

## Observações sobre o Claude
- Cada conversa começa do zero; este arquivo é a memória do projeto.
- O Claude no chat não acessa minha máquina. Para editar o repositório direto, usar o Claude Code.

## 2026-10-29 — tiro no lado errado (run individual 21:02–21:04)
- Evidência: `logs/execucao_20261029.txt` e `logs/tiros/`. Nos turnos 2, 4, 5 e 6 o bot leu `eu x=1200`
  com o personagem em x≈961: o ícone azul do item no painel (1200, 726), abaixo da linha das barras,
  foi aceito como triângulo da vez quando o triângulo real piscava. Com o "eu" errado, alvo e lado saíam errados.
- Correção: `_triangulo` só aceita candidato ~106 px acima de alguma barra de vida (`TRIANGULO_TOLERANCIA`).
  Teste com os prints do registro em `tests/imagens/`; leitura dos prints de batalha da especificação inalterada.
- Convites no estágio 2 não acontecem no `rodar_bot.bat` (uma conta); só em `rodar_grupo.bat`.

## 2026-10-29 — grupo com 3 convidadas (run 21:26–21:30)
- Só 2 convites: faltava `img/convidados/fakeddt1.png` (convite é por nome). Recortado do print
  da aba Amigos enviado pelo usuário (mesma escala do jogo: os outros recortes casam com 1,00).
- Depois do estágio 1 a principal convidava só após visitar cada convidada para ver quem estava
  fora da sala (~13 s). Agora convida direto (`convidar_direto`): as convidadas foram expulsas
  antes do estágio 1, então estão fora. A visita às convidadas continua só para Confirmar e Pronto,
  que o jogo exige (janela coberta não desenha o convite).

## 2026-10-29 — 5 pedidos do usuário (batalha e grupo)
1. Estágio 1: a principal passa (PASS) a 1ª vez que joga; depois ângulo 30, força 10
   (`passar_primeiro_turno` nas estratégias do estágio 1; vale de novo a cada luta).
2. Ângulo ajustado fora da vez, uma vez por estágio (`Batalha.preparar_angulo`), no bot
   individual e na principal do grupo. Se o jogo ignorar ↑/↓ fora da vez, desiste em ~3 toques.
3. Oscilação 29/30: o ajuste tem limite de `ANGULO_TEMPO_MAX` (5 s) e, depois de 2 inversões
   de direção a 1° do alvo, aceita e atira. Antes podia gastar até ~36 s e perder a vez.
4. Sala do estágio 2: depois de aceitar o convite, uma visita por convidada para o Pronto
   (`dar_pronto_na_sala`); convidada com Pronto confirmado não é visitada de novo.
5. Antes do estágio 1 cada convidada clica em Sair (`img/sala_sair.png`) até o Cais e espera
   o convite. Sair só é clicado com a tela reconhecida como sala (no lobby nunca). Convidada
   numa sala sem ter aceitado o convite da principal nesta volta também sai para o Cais.
Testes: `tests/test_batalha_turno.py`, `tests/test_grupo_sala.py`. Não testado no jogo.

## 2026-10-29 — captcha: "Destino ambíguo/ausente" (teclado em roda)
- Causa: o teclado do captcha é uma roda que gira a cada desafio; os botões mudam de lugar e
  de tamanho (embaixo maiores). Os recortes de cada número têm o tamanho da imagem 38 e só
  casavam quando o número caía no mesmo lugar.
- Correção (`teclas_pela_roda`, parâmetros em `perfil.json` → `ring`): círculo de centro
  (135, 115) e raio 100 no teclado, botões a 36°; números decrescentes no sentido horário.
  Ângulo de partida pelos botões pequenos; rotação pela soma das 10 leituras, com folga
  mínima (sem folga não clica). O método antigo continua sendo tentado primeiro.
- Conferido no print do usuário (`tests/imagens/captcha_roda_girada.webp`): 10 de 10 teclas.
- OCR desse captcha (4 4 8 3, dígitos ocos, 3 preto) não gera candidato: o bot não envia nada.
  Camada preta testada e desfeita (o modelo devolve vazio para dígitos ocos). Pendente.
- 29/10: Resetar só limpa as casas digitadas; o código do captcha não muda (resposta do usuário).

## 2026-10-29 — captcha: roda girando (informações do usuário)
- O código só troca quando o tempo acaba ou quando os 4 dígitos enviados estão errados.
- A roda dos números gira sozinha. Com o mouse em cima de uma tecla os outros números somem;
  com o mouse perto (fora dos números) ela gira mais devagar.
- Antes, o clique exigia as 10 teclas no mesmo lugar do print anterior: com a roda girando
  nunca clicaria, e o mouse ia para o canto (roda na velocidade máxima).
- Agora: mouse estacionado no centro da roda antes de cada print e depois de cada clique;
  o número é localizado no print mais recente e adiantado pelo giro medido entre dois prints
  (`VERIFICACAO_LATENCIA_CLIQUE`); acima de `VERIFICACAO_GIRO_MAX` °/s não clica. O log
  mostra "roda +N°/s" a cada número, para calibrar com a próxima run.

## 2026-10-29 — run com o captcha: mouse no centro escondia os botões
- Log: "Teclado não reconhecido (0 botões soltos)", depois "Roda ambígua" e "Botões fora do
  círculo": com o mouse no centro da roda os botões somem/clareiam. O print do usuário (mouse
  fora) é lido certo (10 de 10). Agora o mouse espera na barra do título (`perfil.json` →
  `rest`), fora do teclado. A previsão do giro compensa a roda mais rápida.
- OCR desse captcha (6418: 6 e 8 ocos, 1 preto): só o modelo beta lê 6418 (0,97–0,98); o
  padrão lê "61" e a leitura por cores perde o 1 preto. Sem 2 fontes, não envia. Camada preta
  testada de novo (dá "4136", errado) e desfeita. Precisa de capturas reais sem compressão
  (`logs/verificacao/`).

## 2026-10-29 23:30 — run gravada (vídeo em logs/gravacao_tela/)
- Causa real do "0 botões soltos": o jogo estava SEM FOCO. O aviso "Clique no jogo, para
  continuar jogando" escurece a tela e cobre o teclado; o controlador nunca clica no centro
  para recuperar foco, então esperava para sempre. Agora, com o aviso sobre o captcha, clica
  na barra do título do quadro (fora do teclado e dos botões; `ponto_para_foco`).
- Captchas das secundárias: nenhum foi tratado. O controlador é serial e ficou preso no da
  principal. O log mostra "5 contas: principal + 4 convidada(s)": há uma 5ª janela com o
  título do jogo aberta.

## 2026-10-29 23:39 — 2ª run gravada (usuário mexendo o mouse perto dos números)
- Foco: o clique na barra do título funcionou; o bot passou a ler o captcha.
- Roda (vídeo, 8 fps): ~90–160°/s com o mouse longe; ~5–35°/s com o mouse perto dos números.
  A análise entre os dois prints deixava o giro ambíguo: agora os dois prints são seguidos.
- Clique: o mouse vai para o número previsto (com hover a roda quase para), espera
  `VERIFICACAO_ESPERA_HOVER`, olha de novo e clica no centro do botão sob o mouse
  (`conferir`); sem botão perto, não clica. A identidade do número vem da rotação da roda:
  um botão sozinho é mal classificado (o 9 parecia 8/0 no print real).
- OCR: a unanimidade vetava códigos reais certos (5936 com 7 de 8 leituras; 3828 com 3 de 4).
  Nova regra de maioria em `decidir` (≥ 75% das leituras de 4 dígitos, 2 modelos). Reais:
  3 certos de 8 (antes 1), 0 errados. Sintéticos semente 6: até +1 erro em 150 (difícil).
  Recortes reais rotulados em `tests/imagens/codigo_real_*.png`.

## 2026-10-30 00:00 — captcha nas secundárias e saída para o Cais
- Primeira vez que o bot clicou números (conta2). Roda a −91…−280°/s com o mouse na barra do
  título: o mouse agora espera dentro do painel do teclado, no canto de cima (`rest`), perto dos
  números sem ficar em cima; o clique de foco continua na barra do título (`focus_click`).
- "Botões fora do círculo" com números nítidos: botões grandes de baixo partidos em pedaços
  (mais altos que largos) entravam na conta. Agora só formas largas (`aspect_min`) e pontos
  fora do padrão são descartados em vez de recusar a roda. Teste: `captcha_botao_partido.png`.
- Saída para o Cais (run 23:57): 6 cliques em Sair sem efeito. O recorte casa na palavra
  "SAIR"; o certo é clicar na porta, 27 px acima (`SAIR_PORTA`, informado pelo usuário).
- Ajuste pedido pelo usuário: o mouse espera por dentro da roda, 75 px acima do centro (`rest`).
  Os botões passam a ~25–30 px dele (roda mais lenta) e nunca por cima, em qualquer rotação.

## 2026-10-30 00:24 — ciclo jogado pelo usuário (logs/gravacao_jogada/, com cliques e teclas)
Comparado com o bot e confirmado pelo usuário:
- Pronto das convidadas continua necessário (uma visita por convidada, como já fazia).
- Pandora: a principal mata no 1º turno; as convidadas não ligam o Confiar e o bot não troca
  de janela durante a luta (`CONFIAR_CONVIDADAS = False`).
- Depois da Pandora a principal expulsa as convidadas pelo X do cartão, sem visitar as janelas
  delas (a ida de cada uma ao Cais pela porta Sair saiu desta etapa).
- Antes de recomeçar o estágio 1, o bot confere e resolve os captchas de todas as contas
  (`resolver_codigos`).
- Baú de recompensa: o usuário preferiu deixar como estava (sem clique).
- Ajuste (usuário, 30/10): a conferência de captcha antes do estágio 1 lê as janelas em segundo
  plano; não traz as secundárias para frente. Elas só são visitadas no convite do estágio 2 ou com captcha.

## 2026-10-30 — Pandora: recuo medido
- Run 01:15: nasceu em +186 do rótulo "Força" (alto do morro), ângulo preso em 41; o recuo às cegas
  (6× 0,4 s) não saiu do lugar. Prints do usuário: em +104 o ângulo 30 dá. Agora, na Pandora, se
  nascer além de +150 recua em passinhos medindo a posição até +105 (`recuar` na estratégia).
- Só nesse lugar (+160 a +215): nascendo em outros pontos (-88, +42, +116, +351) não anda.
- Fora desse lugar: só mira 30 e atira com força 80 (`andar: None`); o recuo às cegas foi desligado.
