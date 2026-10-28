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

## Estado atual
- Estrutura do repositório criada (`src/`, `tools/`, `img/`, `logs/`).
- Bot tem só os estados LOBBY e SALA; BATALHA e LOOT estão como TODO.
- Ainda não rodei no jogo. Falta:
  - [ ] Título da janela do cliente: `_______________`
  - [ ] Rodar `python tools/listar_janelas.py` e anotar o título/hwnd.
  - [ ] Recortar `img/botao_iniciar.png` e testar com `tools/testar_template.py`.
  - [ ] Conferir se o círculo vermelho em `logs/teste_template.png` caiu no botão.

## Próximos passos
1. Fazer o bot achar e clicar em **1 botão** de ponta a ponta.
2. Adicionar um estado por vez (LOBBY -> SALA -> BATALHA -> LOOT).
3. Só depois: lógica de turno e mira.
4. Só depois: Fase 2 (segundo plano) e Fase 3 (várias instâncias).

## Dicas aprendidas
- Fixe resolução e posição da janela, senão os templates deixam de bater.
- Adicione atrasos aleatórios entre ações.
- Salve screenshots (`logs/`) quando o bot se perder, para depurar.
- Mantenha `.env` fora do Git se um dia guardar credenciais.

## Como rodar
```bash
python -m venv .venv
.venv\Scripts\activate
pip install -r requirements.txt
python tools/listar_janelas.py
python tools/testar_template.py img/botao_iniciar.png "TITULO DA JANELA"
python -m src.main "TITULO DA JANELA"
```

## Observações sobre o Claude
- Cada conversa começa do zero; este arquivo é a memória do projeto.
- O Claude no chat não acessa minha máquina. Para editar o repositório direto, usar o Claude Code.
