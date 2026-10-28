# Parte do título da janela do jogo. Deixe vazio para escolher na hora.
TITULO_JANELA = "DDClássico"

# Confiança mínima (0 a 1) para considerar que um template foi encontrado.
LIMIAR = 0.85

# Segundos de espera depois dos cliques de uma tela, para a tela trocar.
ESPERA_TRANSICAO = (1.5, 2.5)

# Segundos de espera entre cliques seguidos na mesma tela (ex.: Formigueiro -> Normal -> ok).
ESPERA_CLIQUE = (0.6, 1.0)

# Até quantos segundos esperar um botão aparecer (popups demoram um pouco para abrir).
ESPERA_ALVO = 5

# Quantas vezes seguidas a mesma tela pode aparecer antes de avisar que o clique não funcionou.
REPETICOES_MAX = 5

# Quantos ciclos sem reconhecer nenhuma tela antes de salvar um print em logs/.
SEM_TELA_MAX = 10

# Salva em logs/ um print antes de cada clique e outro depois do último (para depurar).
SALVAR_PRINTS = True

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
                 "forca_colado": 1, "colado_px": 25},
    # Estágio 2: a Pandora (rainha) fica à direita; anda para trás (←) se o ângulo 30 não der.
    # Tecla 3 = "+2", tecla 9 = dano "max".
    "PANDORA": {"angulo": 30, "forca": 80, "skills": ["3", "9"], "alvo": "direita", "andar": "esquerda"},
}

# Parar o bot quando a instância acabar (saiu da batalha depois do último estágio).
# False = começar outra instância sozinho.
PARAR_DEPOIS_DA_INSTANCIA = True
ULTIMO_ESTAGIO = "PANDORA"

# Andar: quanto tempo segurar a seta em cada passo e quantos passos tentar no máximo.
PASSO_ANDAR = 0.4
MAX_PASSOS = 6

# O bot lê o ângulo pela agulha do relógio e aperta ↑/↓ até chegar no ângulo da estratégia.
# A leitura erra ~1°; False = não mexer no ângulo (você ajusta à mão).
AJUSTAR_ANGULO = True
ANGULO_TOLERANCIA = 1.5
TECLA_ANGULO_SOBE = "up"
TECLA_ANGULO_DESCE = "down"

# Segundos de pausa quando a batalha começa, antes do 1º tiro.
PAUSA_INICIO_BATALHA = 3

# Quantos pixels valem 1 de "distância" das tabelas (a régua verde 0–10 cobre a tela de ~1000 px).
PX_POR_DISTANCIA = 100

# Solta o Espaço um pouco antes da força pedida, para compensar o atraso da leitura.
FORCA_ANTECIPACAO = 1.0

# Segurança: solta o Espaço depois deste tempo, mesmo sem chegar na força.
TEMPO_MAX_FORCA = 5

TECLA_ATIRAR = "space"
TECLA_ESQUERDA = "left"
TECLA_DIREITA = "right"
