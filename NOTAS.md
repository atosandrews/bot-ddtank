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

## Próximos passos
1. Especificar a batalha (detectar a vez, mira, tiro) e o loot, com prints.
2. Fazer o bot voltar sozinho ao início do fluxo depois do loot (loop de instâncias).
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
