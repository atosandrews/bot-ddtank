# Login headless: transporte, recuperação e validação

Implementação preparada em 13/11/2026. Testes offline não comprovam aceitação pelo Cloudflare nem login no jogo. O perfil `launcher_chromium87` é um candidato fixo de compatibilidade; ainda precisa ser comparado com uma sessão aceita do launcher e validado no Fly.

## Configuração

O YAML aceita um bloco opcional, mantendo `session_file` e `characters` existentes:

```yaml
auth:
  transport: standard
  status_file: /tmp/ddctl/auth-main.json
  coordination_dir: /tmp/dd-auth-coordination
```

`standard` é o padrão compatível com configurações anteriores. `launcher_chromium87` usa `tls-client v1.16.0`, ClientHello Chrome 87 e parâmetros HTTP/2 fixos. Continua validando certificados; não executa JavaScript nem desafios interativos. O protocolo RSA/MD5 e XOR/zlib do jogo foi preservado.

Os caminhos relativos no YAML são resolvidos em relação ao próprio YAML. As opções de CLI `-transport`, `-status-file` e `-coordination-dir` substituem esses valores; caminhos relativos da CLI usam o diretório atual. A pasta de estado/coordenação deve ser privada (`0700`); snapshots e arquivos de sessão são privados (`0600`).

O arquivo `.Auth` é relido imediatamente antes de cada tentativa HTTP permitida. O valor do arquivo é a autoridade para o host do site; o jar não acrescenta outro `.Auth` antigo naquele pedido. Outros cookies conservam seu escopo de domínio e expiração. O renovador externo continua responsável por persistir cookies renovados.

## Diagnóstico e promoção do transporte

No contêiner Linux, a partir de `/ws/ddrelay`, use uma configuração de **um personagem desconectado**:

```sh
go run ./cmd/ddrelay login -config /tmp/cfg/headless-bot.yaml -check-auth -transport launcher_chromium87 -coordination-dir /tmp/dd-auth-coordination
```

`-check-auth` executa as duas etapas HTTP uma vez por personagem e encerra sem abrir TCP do jogo. Respeita o cooldown compartilhado. Para comparar, execute o mesmo comando com `-transport standard`, mantendo conta, IP e endpoints. Aguarde o cooldown informado em caso de bloqueio.

Os eventos `headless HTTP request` registram etapa, host, caminho sem query, status, protocolo, duração, Ray ID e classificação. Cookies, bilhetes, senha temporária, `p`, `key` e corpos completos não são registrados. O host `resource` não precisa ser baixado pelo autenticador; acesso a arquivo público é somente diagnóstico auxiliar.

A aprovação tem três critérios distintos:

1. Resposta HTTP acessível: um arquivo público `200` comprova acesso somente àquele recurso.
2. Autenticação HTTP: XML aceito e ID do personagem correspondente à configuração.
3. Sessão do jogo: confirmação TCP do personagem, primeiro heartbeat e sessão `game_ready` no relay.

Depois do diagnóstico, iniciar o login normal com `-transport launcher_chromium87` exige cumprir o terceiro critério. Validar três ciclos com uma conta desconectada, um grupo por 30 minutos e os demais por duas horas, incluindo recuperação após desconexão. Preservar inicialmente o upstream TCP configurado. Se o candidato continuar bloqueado, registrar a incompatibilidade; a contenção de reinícios não significa login corrigido.

## Recuperação e estado

A inicialização do processo não espera o login remoto. Workers independentes autenticam e reconectam personagens, preservando as conexões saudáveis. Cada tentativa de autenticação tem prazo de 60 segundos, com 20 segundos por pedido HTTP. As esperas aceitam cancelamento e o encerramento fecha as conexões.

Bloqueio reconhecido, desafio ou `403` desconhecido acionam cooldown compartilhado de 5, 10 e depois 15 minutos, com jitter positivo de até 10%. `Retry-After` pode aumentar a espera. Um sucesso completo no login HTTP limpa o circuito. Falhas transitórias usam espera de 5 segundos até 1 minuto, sem desistência automática após dez minutos.

Sessão ausente, inválida, expirada ou login rejeitado passam para `session_required`: o processo aguarda alteração válida do cookie, sem repetir pedidos HTTP. Rejeição de identidade do personagem é erro de configuração. Falhas operacionais de escrita/coordenação permanecem erros explícitos.

O JSON de estado tem `version: 1`, `pid`, `updated_at` e uma lista `characters` com `id`, `phase`, `reason`, `next_attempt_at` e `attempt_deadline` quando aplicáveis. As fases são `queued`, `authenticating`, `connecting`, `online`, `blocked`, `backoff`, `session_required`, `failed` e `stopped`. O heartbeat é atualizado a cada dez segundos, inclusive durante cooldown e espera por sessão. `online` neste arquivo indica confirmação do peer; a prontidão do relay é consultada separadamente.

`auth_watchdog.py` recebe um ou mais snapshots:

```sh
python3 -I /ws/run/auth_watchdog.py /tmp/ddctl/auth-main.json /tmp/ddctl/auth-guests.json
```

- Código `0`: recuperação/espera válida; suspender gatilhos de jogo dependentes dos personagens ausentes.
- Código `1`: todos online; aplicar os gatilhos normais de progresso.
- Código `2`: estado ausente, inválido, vencido, parado ou tentativa travada; tratar como falha operacional.

Heartbeat vence em 60 segundos. Uma transição de retry aceita até dez segundos de atraso; tentativa em andamento deve ter prazo próprio válido. O supervisor verifica também a existência dos processos e a disponibilidade do controle do relay, independentemente do estado de autenticação. Após recuperação, reinicializa os contadores de falta de personagens, sala, confirmações e progresso. Esgotamento da fila de consultas continua sendo detectado.

Os scripts locais publicam `auth-main.json` e, no Pintinho, também `auth-guests.json`. `AUTH_TRANSPORT` seleciona o transporte; o padrão permanece `standard`. `AUTH_COORDINATION_DIR` substitui a pasta compartilhada. Locks Linux coordenam processos que compartilham esse filesystem; contêineres e máquinas separados têm circuitos independentes. A pasta padrão fica fora da limpeza dos scripts e sobrevive ao reinício do mesmo contêiner, mas não à sua substituição. Use armazenamento montado compartilhado quando essa persistência for necessária.

O painel usa prontidão corrente do relay para contar online. Logs históricos servem apenas aos horários; um login antigo não comprova conexão atual.

## Integração pendente no Fly

Este checkout não contém o `iniciar.sh` nem os manifestos usados no Fly. Antes de implantar, obter e comparar esses arquivos com a produção. Aplicar neles as mesmas verificações dos scripts locais: PID, controle limitado por timeout, snapshots de todos os processos, suspensão dos gatilhos dependentes e reinicialização dos contadores na recuperação.

Separar saúde do processo e prontidão do jogo nas verificações da implantação. Não promover o transporte enquanto faltar XML aceito e prontidão TCP observada. Manter a imagem anterior para rollback. Nenhum deploy, chamada ao site ou teste real do jogo integra a validação offline descrita aqui.

## Verificação offline

No workspace `headless/codigo`, em Linux com Go 1.26:

```sh
go test ./ddrelay/... ./api/... ./bot/...
go test -race ./ddrelay/...
```

Na raiz deste repositório, com Python 3.13 do ambiente próprio:

```powershell
.venv/Scripts/python.exe -I -m unittest discover -s headless -p test_painel_auth.py
.venv/Scripts/python.exe -I -m unittest discover -s headless/codigo/run -p 'test_*auth*.py'
```

Os testes do supervisor executam somente seus loops com comandos simulados, usando `sh` ou a imagem Docker `golang:1.26` sem rede. Não execute os scripts completos para validar offline.
