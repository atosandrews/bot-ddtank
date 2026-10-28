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
- Ferramenta de Captura do Windows travou com 32 cópias abertas (não era o bot); fechadas.
  O bot agora solta Espaço e setas sempre que encerra.
- SALA_PRONTA não é mais o fim: depois do Início o bot segue para a batalha. O fim da batalha
  e o loot ainda não são reconhecidos (o bot fica esperando e salva print em logs/).

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
