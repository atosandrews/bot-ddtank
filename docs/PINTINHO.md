# Pintinho Corre Rápido (Difícil)

Script separado do Formigueiro: `python -m src.pintinho` ou `rodar_pintinho.bat`.
Código: [src/pintinho.py](../src/pintinho.py) (fluxo e tiros) e
[src/pintinho_visao.py](../src/pintinho_visao.py) (o que o bot lê). Templates em `img/pintinho/`.

## Regras do usuário (30/10/2026)

- 4 estágios. Por enquanto **uma conta só** faz tudo. Depois: a principal faz 1–3 sozinha e só
  no **estágio 4** convida as outras 3; no fim, expulsa e recomeça (como no Formigueiro).
- Itens em **todo tiro**: tecla **3** (+2) e tecla **9** (dano máximo).
- O 1º turno dos estágios **1 e 3** não é tiro: é **Voar (F)**, para chegar perto dos pintinhos /
  do Grange.
- Estágio 2: **nunca pode errar**. Se errar uma galinha, elas usam **escudo**, o estágio trava
  e o bot pode ficar em loop infinito.
- Estágio 2 (galinhas, à direita), varia com o pixel em que nasceu e com o vento:
  - de baixo: ângulo 25, força 60;
  - do meio: ângulo 45, força 58–62;
  - de cima: ângulo 55, força 68–72.
- Última sala (estágio 4): **sempre ângulo 20, força 75**; sem ângulo 20 no lugar, andar um pouco
  até ter.
- Fim da instância: aba de cartas, depois volta à sala ("Selecione a instância").
- A 1ª volta da gravação foi diferente; a partir da 2ª é o padrão.
- Respostas de 30/10 (2ª rodada):
  - galinha de baixo: nascendo mais atrás 30/62, mais à frente 25/60; **prefere que o bot
    calcule pela tabela**;
  - estágio 4: **sempre 20/75**, também no 1º tiro; sem ângulo 20, anda só o necessário;
  - estágio 3: depois do voo, **anda um pouco para trás da galinha** para pegar a galinha e a
    casa no mesmo tiro: **30/20, sempre para o lado da casa**;
  - estágio 1: depois do voo 40/10; **pintinho quase colado: força 5** (como no Formigueiro).
  - errar uma galinha: ela põe escudo e depois tira; não precisa recomeçar.

## Gravação de 30/10 (`logs/gravacao/20261030_100744`)

Cliente 1920x1032 (área interna da janela), DPI 100%. 3 instâncias completas, 824 prints,
gravadas com [tools/gravar_sessao.py](../tools/gravar_sessao.py). Os prints não estão no git
(`logs/` é ignorado).

Fluxo observado: Lobby → Cais → sala → Configuração da sala (Pintinho → Difícil → ok) →
Início → "lutar sozinho?" OK → batalha (popup "Requisitos/Vitória": fechado no X) → fim do
estágio ("Resultado Tempo" e 9 cartas: virou 1) → sala com "STAGE 2" → Início → … → fim do
estágio 4 (21 cartas: 2 grátis; a 3ª é só do Clube Prime) → sala com "Selecione a instância".

| Estágio | Missão (quadro de cima à direita) | Tiros das voltas 2 e 3 |
|---|---|---|
| 1 | "Derrote os pintinhos" (0/30) | 1º: 41°/56 para o lado dos pintinhos; depois 30–41°/~10 no mais perto |
| 2 | "Derrote 3 Galinhas" (0/3) | 30°/62, 44°/60–62, 55°/68–70, todos para a direita |
| 3 | "Destrua o Grange" (0/1) | 1º: 41°/62; depois 50–51°/17–22, para a esquerda |
| 4 | "Destrua a Gaiola" (0/1) | 22°/91 no 1º (ângulo baixo ali), depois 20°/73–78 para a esquerda; último tiro ~29°/30–41 para a direita |

Vento do estágio 2 em cada tiro (sinal + = soprando para a direita, a favor do tiro):

| Volta | De baixo | Do meio | De cima |
|---|---|---|---|
| 1 | 25°/70, vento −1,2 | 44°/63, −2,5 | 55°/71, −2,4 |
| 2 | 30°/62, +3,0 | 44°/60, +2,0 | 55°/68, +2,7 |
| 3 | 30°/62, +1,2 | 44°/62, +3,0 | 55°/70, +3,9 |

## O que o bot faz

- **Estágio**: lido no quadro de missão (texto amarelo), não pela ordem das lutas.
- **Estágio 1**: 1º turno **voa (F)** com 41°/56 para o lado dos pintinhos no minimapa (ponto azul = eu,
  vermelhos = inimigos). Depois: 40°/10 no mais perto da tela; a até 30 px (quase colado) ou
  sem barra à vista (escondida atrás da minha), **40°/5**, para o lado do minimapa.
- **Estágio 2**: o contador "n/3" escolhe a galinha (0 mortas → de baixo, 1 → do meio,
  2 → de cima); errar o tiro mantém a mesma galinha. Ângulos 30, 45 e 55.
  **Força pela tabela da moldura do jogo** (Âng. 20/30/50/65; 45 e 55 interpolam entre as
  vizinhas) com a **distância medida no minimapa**: a moldura cinza da câmera mede ~86,5 px
  para ~1000 px da tela, e 100 px = 1 de distância. Galinha da vez = a mais baixa das que
  sobraram no minimapa. Ponto de partida = o tiro que acertou na gravação (força sem vento
  62,2 / 62,5 / 70,6 a 114 / 116 / 121 px no minimapa); a tabela soma ou tira conforme o
  pixel em que nasceu. **Vento: 0,17 de força por unidade** (contra soma, a favor tira; no
  máximo ±3) — é o que as 3 voltas mostram (com 0,5 a volta 1 e a 3 não batiam).
  Resultado contra a gravação: até 1 de força dos tiros do usuário, menos a galinha de baixo
  da volta 2 (58,7 contra 62, e as duas forças acertaram nela em voltas diferentes).
  Para não errar: contador, vento e minimapa lidos em até 8 prints; só atira com o relógio
  mostrando o ângulo **exato** (senão ajusta de novo e, se ainda não der, passa a vez).
  Se errar (contador não subiu no turno seguinte), salva o print `errou_galinha` e mira na
  mesma galinha de novo: o escudo dela some sozinho (usuário, 30/10).
  Para a galinha de baixo o bot usa **30°** (o que acertou nas voltas 2 e 3), não 25/60.
- **Estágio 3**: 1º turno **voa (F)** com 41°/62. No turno seguinte **recua 0,45 s** (para a
  direita, como nas 3 voltas: 0,42–0,55 s), vira de volta e atira **30°/20 para a esquerda**
  (lado da casa); depois, 30°/20 para a esquerda sem andar.
- **Estágio 4**: o alvo da esquerda é o **Burrinho**, que protege a gaiola com um escudo.
  **Sempre 20°/75**, também no 1º tiro. Se o ângulo travar, passinhos de
  0,15 s para trás (até 3) e depois para frente (até 6), virando para ele e tentando o 20 a
  cada um. Quando a barra dele no alto (ao lado do vento) fica sem vermelho, o escudo caiu
  ("Excelente! O escudo de proteção sumiu…"): tiro na **gaiola, à direita** (ponto verde no
  minimapa), **tabela de 30° pela distância + 5,8** (as 3 voltas deram +5,3, +6,3 e +5,9) e
  o vento; errou, a próxima tentativa soma +3, −3, +6.
  No Burrinho e no Grange a tabela não bate com os tiros que acertaram (alvos grandes: na
  volta 3 o Burrinho estava a 11,5 de distância e na volta 2 a 14,7, e 74–78 acertou nas
  duas); ficam os valores fixos do usuário.
- **Cartas**: 1 entre os estágios, 2 no fim. **Popup** "Requisitos/Vitória": fechado no X.

## Validação offline (30/10)

Sem o jogo, sobre os 824 prints da gravação: estágio certo em 680 de 681 prints de batalha
(1 sem leitura, nenhum errado); contador de galinhas sem erro nos lidos; vento certo nos 28
casos conferidos à mão; o plano de tiro de cada turno comparado com o que o usuário fez
(ver tabela acima). Testes: `tests/test_pintinho.py`, com recortes em `tests/imagens/pintinho/`.
**Ainda não rodou no jogo.**

## Ainda não coberto

- A tabela em pixels de nascimento novos: na gravação só apareceram dois (114 e 105 px da
  galinha de baixo).
- Dígitos do vento 6 e 8 não apareceram na gravação: com eles o vento fica ilegível e o bot
  atira sem correção (registra no log).
- Grupo (convidar as 3 contas no estágio 4): a sala mostra "STAGE N", que vai dizer a hora.
