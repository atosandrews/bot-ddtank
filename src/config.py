# Parte do título da janela do jogo. Deixe vazio para escolher na hora.
TITULO_JANELA = "DDClássico"

# Verificação automática: SDK local ddddocr, sem limite de tentativas por desafio.
VERIFICACAO_LIMIAR_TITULO = 0.85
VERIFICACAO_LIMIAR_OCR = 0.90  # heurística do modelo; não é probabilidade de acerto
VERIFICACAO_LIMIAR_TECLAS = 0.90
VERIFICACAO_MARGEM_TECLAS = 0.03
VERIFICACAO_INTERVALO = 1.0
VERIFICACAO_ESPERA_CLIQUE = 0.25
VERIFICACAO_ESPERA_RESPOSTA = 2.5
VERIFICACAO_DURACAO_DESAFIO = 30.0
# A roda dos números gira. Acima desta velocidade (°/s), medida entre dois prints, não clica.
VERIFICACAO_GIRO_MAX = 300.0  # vídeo de 29/10: ~90–160°/s com o mouse longe
# Espera (s) com o mouse em cima do número antes de conferir e clicar (a roda quase para).
VERIFICACAO_ESPERA_HOVER = 0.15
# Tempo (s) entre o 2º print e o clique de fato (mover o mouse e clicar), para adiantar o giro.
VERIFICACAO_LATENCIA_CLIQUE = 0.12
VERIFICACAO_SALVAR_DIAGNOSTICO = True  # grava recortes em logs/verificacao/ para calibrar o OCR

# Confiança mínima (0 a 1) para considerar que um template foi encontrado.
LIMIAR = 0.85

# Segundos de espera depois dos cliques de uma tela, para a tela trocar.
# Faixas com sorteio: parece uma pessoa (nunca o mesmo tempo exato).
ESPERA_TRANSICAO = (0.8, 1.3)

# Segundos de espera entre cliques seguidos na mesma tela (ex.: Formigueiro -> Normal -> ok).
ESPERA_CLIQUE = (0.25, 0.45)

# Até quantos segundos esperar um botão aparecer (popups demoram um pouco para abrir).
ESPERA_ALVO = 5

# Quantas vezes seguidas a mesma tela pode aparecer antes de avisar que o clique não funcionou.
REPETICOES_MAX = 5

# Quantos ciclos sem reconhecer nenhuma tela antes de salvar um print em logs/.
SEM_TELA_MAX = 10

# Salva em logs/ um print antes de cada clique e outro depois do último (para depurar).
SALVAR_PRINTS = True

# Salva em logs/tiros/ um print anotado (JPEG) de cada tiro: eu, formigas, alvo e lado.
SALVAR_PRINT_TIRO = True

# ---------------- Batalha ----------------

# Como jogar cada estágio. O estágio é reconhecido pelo quadro de missão ("Derrote ...").
#   angulo: ângulo do tiro (com forca None, use 20, 30, 50 ou 65: as tabelas da moldura)
#   forca:  força fixa; None = calcular pela distância e pela tabela do ângulo
#   skills: teclas dos itens (coluna da direita, 1 a 9) apertadas antes de cada tiro;
#           sem energia para um item, o jogo simplesmente ignora a tecla
#   alvo:   "perto" = vira para o inimigo mais perto; "direita"/"esquerda" = lado fixo
#   andar:  se o chão não deixar chegar no ângulo, anda para esse lado e tenta de novo
#   forca_colado / colado_px: força usada quando o inimigo está colado no personagem
#           (barra dele a até colado_px pixels, ou escondida atrás da minha barra)
ESTRATEGIAS = {
    # Estágio 1: as formigas vêm até o personagem. Formiga colada em cima dele: força 1.
    "FORMIGAS": {"angulo": 30, "forca": 10, "skills": [], "alvo": "perto", "andar": None,
                 "forca_colado": 1, "colado_px": 25,
                 # A 1ª vez que joga no estágio 1, só passa (pedido do usuário, 29/10).
                 "passar_primeiro_turno": True},
    # Estágio 2: a Pandora (rainha) fica à direita; anda para trás (←) se o ângulo 30 não der.
    # Tecla 3 = "+2", tecla 9 = dano "max".
    "PANDORA": {"angulo": 30, "forca": 80, "skills": ["3", "9"], "alvo": "direita", "andar": "esquerda"},
}

# ---------------- Grupo (várias contas, python -m src.grupo) ----------------
# A conta principal é a janela maior; as outras são as convidadas.
# Estágio 1: as duas contas mais próximas formam uma dupla, juntam no mesmo pixel e a dupla
# da esquerda atira para a esquerda, a da direita para a direita. Tecla 9 em todo tiro.
ESTRATEGIAS_GRUPO = {
    # Em grupo é sempre força 10, mesmo com formiga colada (pedido do usuário).
    "FORMIGAS": {"angulo": 30, "forca": 10, "skills": ["9"], "alvo": "dupla", "andar": None,
                 "juntar": True},
    "PANDORA": ESTRATEGIAS["PANDORA"],
}
# True: na batalha só a principal joga; as convidadas ligam o "Confiar" do jogo (o jogo
# joga por elas) uma vez por estágio. Evita trocar de janela a cada turno.
SO_PRINCIPAL_JOGA = True
# True: o estágio 1 a principal faz sozinha (padrão de uma conta); só para o estágio 2
# (Pandora) ela convida as outras, que ficam no Confiar. Depois da Pandora elas saem da sala.
CONVIDAR_SO_NO_ESTAGIO_2 = True
# False: na Pandora as convidadas não ligam o Confiar; a principal a mata no 1º turno e o bot
# não troca de janela durante a luta (usuário, 30/10). True: visita cada uma para o Confiar.
CONFIAR_CONVIDADAS = False
# Estratégia da principal nesse modo: anda até ficar colada nas outras contas (as formigas
# vêm todas juntas), atira na formiga mais perto, sempre força 10 e tecla 9.
ESTRATEGIAS_SO_PRINCIPAL = {
    # A principal fica parada (as convidadas vêm até ela). Só ela ataca e as formigas vêm
    # dos dois lados: alterna o lado a cada turno.
    "FORMIGAS": {"angulo": 30, "forca": 10, "skills": ["9"], "alvo": "perto", "andar": None,
                 "alternar_lados": True, "passar_primeiro_turno": True},
    "PANDORA": ESTRATEGIAS["PANDORA"],
}
DUPLA_MAX_PX = 150      # mais longe que isso, a conta mais próxima não é da minha dupla
JUNTAR_PX = 4           # "mesmo pixel": para de andar quando a dupla estiver a até isso
JUNTAR_TOQUE = 0.06     # segundos de seta em cada passinho até a dupla
JUNTAR_MAX_TOQUES = 30
# Segundos entre convites, se as contas não entrarem na sala.
REENVIAR_CONVITE = 15

# Parar o bot quando a instância acabar (saiu da batalha depois do último estágio).
# False = começar outra instância sozinho.
PARAR_DEPOIS_DA_INSTANCIA = False
ULTIMO_ESTAGIO = "PANDORA"

# Andar: quanto tempo segurar a seta em cada passo e quantos passos tentar no máximo.
PASSO_ANDAR = 0.4
MAX_PASSOS = 6

# O bot lê o ângulo pela agulha do relógio e aperta ↑/↓ até chegar no ângulo da estratégia.
# A leitura erra ~1°; False = não mexer no ângulo (você ajusta à mão).
AJUSTAR_ANGULO = True
ANGULO_TOLERANCIA = 1.5
# Tempo máximo (s) para acertar o ângulo; passado isso atira com ±1° ou desiste do ajuste.
ANGULO_TEMPO_MAX = 5.0
TECLA_ANGULO_SOBE = "up"
TECLA_ANGULO_DESCE = "down"

# Segundos de pausa quando a batalha começa, antes do 1º tiro.
PAUSA_INICIO_BATALHA = 1

# Quantos pixels valem 1 de "distância" das tabelas (a régua verde 0–10 cobre a tela de ~1000 px).
PX_POR_DISTANCIA = 100

# Solta o Espaço um pouco antes da força pedida, para compensar o atraso da leitura.
FORCA_ANTECIPACAO = 1.0

# Segurança: solta o Espaço depois deste tempo, mesmo sem chegar na força.
TEMPO_MAX_FORCA = 5

TECLA_ATIRAR = "space"
TECLA_ESQUERDA = "left"
TECLA_DIREITA = "right"
