# Instruções para agentes — bot-ddtank

Leia o [contexto](docs/CONTEXTO.md), o [guia de desenvolvimento](docs/DESENVOLVIMENTO.md) e o [guia de testes](docs/TESTES.md).

- Papéis: [desenvolvedor](agents/desenvolvedor.md) e [tester](agents/tester.md), também disponíveis como subagentes do Claude Code em `.claude/agents/`. Skills: [catálogo](docs/SKILLS.md), armazenadas em `.agents/skills/`. Projeto irmão OCR: [DDDDOCR.md](docs/DDDDOCR.md).
- Use o código como fonte do comportamento atual. A [especificação ilustrada](docs/especificacao/ESPECIFICACAO.md) cobre a entrada; [NOTAS.md](NOTAS.md) conserva histórico e próximos passos antigos.
- Não execute `src.main`, `src.grupo`, arquivos `.bat` ou ferramentas de captura durante tarefas documentais ou validação offline. Os drivers podem trazer janelas para frente e enviar mouse/teclado reais.
- Preserve controle de foco e liberação das teclas. A verificação é automática por solicitação do usuário: SDK local, cliques verificados, tentativas sem limite fixo e nenhuma repetição do mesmo candidato rejeitado na mesma imagem. Consulte [VERIFICACAO_OCR.md](docs/VERIFICACAO_OCR.md).
- Preserve a precedência de `TELAS` em `src/telas.py`: popups antecedem estados visíveis atrás deles. Ao alterar reconhecimento, adicione screenshot e verifique os alvos de clique.
- Recorte templates sem hover. Registre resolução, escala/DPI e região do cliente; não reduza limiares apenas para fazer um caso positivo passar.
- Separe visão de captura/ação. Para testes offline, passe imagens salvas e substitua drivers/esperas quando necessário. Consulte [TESTES.md](docs/TESTES.md).
- Use `.venv` próprio e versão explícita de Python. A presença de Python 3.13 não comprova compatibilidade das dependências; o launcher padrão observado é 3.15.
- O bot integra o SDK local do projeto irmão `ddddocr`, sem HTTP ou Supabase. Não confundir modelos carregados/testes simulados com resolução validada no jogo.
- Entregue arquivos, motivo, comandos e resultados observados. Diferencie inspeção estática, testes offline e execução no jogo; registros históricos não são resultados novos.
