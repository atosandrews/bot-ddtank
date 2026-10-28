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
