-- Olimpíadas Boom (Normal): Zeconfusso lidera; nas balas (estágio 1) as 4 atacam; nos chefes só ela.
-- barulhoalto, laranjinhatk e isekaiforte ficam na sala a partida inteira.
-- O relay carrega o script logo após o login, antes de o bot conhecer o ID do personagem;
-- por isso a Olimpíadas só é criado quando player.state().id estiver disponível.
-- O líder espera um pouco mais que os convidados, para eles já terem organizado a mochila
-- quando o convite chegar (o convite expira em 30 s).
local AQUECIMENTO = 5
local AQUECIMENTO_LIDER = 20
-- FORTE_ID: ID no jogo do personagem forte (preencher depois do 1º login; ver LEIA-ME.md).
local FORTE_ID = 38878  -- Zeconfusso (website_id 39520)
local party = {
    leader_id = FORTE_ID,
    -- as 3 convidadas ficam na sala do começo ao fim e só passam a vez (pedido do usuário, 05/11)
    members = {FORTE_ID, 121552, 121558, 121562},  -- Zeconfusso, barulhoalto, laranjinhatk, isekaiforte
}

-- Recuperação: a Olimpíadas pausa de vez em estados que pedem conferência manual (convite sem
-- confirmação, comando incerto...). Rodando sozinho, depois de RECUPERAR segundos nesse estado o
-- Olimpiadas é recriado do zero, lendo de novo o estado do servidor; os contadores são somados.
local RECUPERAR = 20
local PARADOS = {paused = true, incomplete = true, configuration_unconfirmed = true}

-- Mochila: toda partida move a bolsa temporária para a mochila. Com a mochila cheia o jogo manda o
-- que não cabe para o Correio; o bot segue na hora, arruma a mochila na coleta seguinte e o painel
-- avisa. Receber os anexos do Correio continua manual (Selecionar tudo > Receber anexos).
local instance
local geracao = 0
local mochila = {itens = 0, equipamentos = 0}
local acumulado = {runs = 0, wins = 0, losses = 0, mailed = 0, undelivered = 0, deliveries = {}}
local recuperacoes = {}
local parado_desde

-- Só reconta quando a mochila muda (inventory_revision): ler a mochila inteira custa CPU (09/11).
local mochila_revisao
local function contar_mochila()
    local revisao = player.state().inventory_revision
    if revisao and revisao == mochila_revisao then return end
    mochila_revisao = revisao
    local itens, equipamentos = 0, 0
    for _, item in ipairs(player.inventory()) do
        if item.bag_type == 1 then
            itens = itens + 1
        elseif item.bag_type == 0 and item.place >= 31 then
            equipamentos = equipamentos + 1
        end
    end
    mochila.itens, mochila.equipamentos = itens, equipamentos
end

local function atual()
    return instance and olimpiadas_status and olimpiadas_status() or {}
end

-- Um Olimpiadas recém-criado pode contar a última luta, que já tinha acabado, como partida nova.
-- Só conta o que ele fizer a partir da primeira luta que viu começar: o que já constava nesse
-- momento fica em "ignorar".
local ignorar

local function marcar_inicio_da_luta(s)
    if ignorar or s.phase ~= "fighting" then return end
    ignorar = {runs = s.completed_runs or 0, wins = s.victories or 0, losses = s.defeats or 0,
        mailed = s.mailed_runs or 0, undelivered = s.undelivered_runs or 0, deliveries = {}}
    for chave in pairs(s.deliveries or {}) do ignorar.deliveries[chave] = true end
end

-- Totais desde o login: a Olimpíadas atual somado aos que foram recriados antes.
local function totais()
    local s = ignorar and atual() or {}
    local base = ignorar or {runs = 0, wins = 0, losses = 0, mailed = 0, undelivered = 0, deliveries = {}}
    local entregas = {}
    for chave, entrega in pairs(acumulado.deliveries) do entregas[chave] = entrega end
    for chave, entrega in pairs(s.deliveries or {}) do
        if not base.deliveries[chave] then entregas[geracao .. ":" .. chave] = entrega end
    end
    return {
        runs = acumulado.runs + (s.completed_runs or 0) - (s.completed_runs and base.runs or 0),
        wins = acumulado.wins + (s.victories or 0) - (s.victories and base.wins or 0),
        losses = acumulado.losses + (s.defeats or 0) - (s.defeats and base.losses or 0),
        mailed = acumulado.mailed + (s.mailed_runs or 0) - (s.mailed_runs and base.mailed or 0),
        undelivered = acumulado.undelivered + (s.undelivered_runs or 0) - (s.undelivered_runs and base.undelivered or 0),
        deliveries = entregas,
    }
end

local function recriar(motivo)
    local t = totais()
    acumulado = {runs = t.runs, wins = t.wins, losses = t.losses, mailed = t.mailed,
        undelivered = t.undelivered, deliveries = t.deliveries}
    table.insert(recuperacoes, 1, string.format("%.0fs %s", session.now(), string.sub(motivo, 1, 120)))
    recuperacoes[11] = nil
    instance = nil
    ignorar = nil
    parado_desde = nil
end

-- Resultado de cada captcha respondido, para medir o OCR com imagens do jogo: quando a revisão
-- muda, a imagem anterior foi aceita (nada pendente) ou recusada (chegou outra imagem).
local captchas = {aceitos = 0, recusados = 0, historico = {}}
local captcha_atual

local function acompanhar_captcha()
    if not captcha then return end
    local estado = captcha.state()
    if captcha_atual and estado.revision ~= captcha_atual.revision then
        local resultado = estado.required and "recusado" or "aceito"
        if resultado == "aceito" then captchas.aceitos = captchas.aceitos + 1 else captchas.recusados = captchas.recusados + 1 end
        table.insert(captchas.historico, 1, {sha256 = captcha_atual.sha256, resultado = resultado, tempo = session.now()})
        captchas.historico[51] = nil
        captcha_atual = nil
    end
    if estado.required and estado.known then
        captcha_atual = {revision = estado.revision, sha256 = estado.image_sha256}
    end
end

local function verificar()
    acompanhar_captcha()
    contar_mochila()
    local s = atual()
    marcar_inicio_da_luta(s)
    if PARADOS[s.phase] then
        parado_desde = parado_desde or session.now()
        if session.now() - parado_desde >= RECUPERAR then recriar(tostring(s.reason)) end
    else
        parado_desde = nil
    end

    local t = totais()
    local avisos = {}
    if t.mailed > 0 then
        avisos[#avisos + 1] = string.format("mochila cheia: itens de %d partida(s) foram para o Correio", t.mailed)
    end
    if t.undelivered > 0 then
        avisos[#avisos + 1] = string.format("%d partida(s) com itens presos na bolsa temporária", t.undelivered)
    end
    mochila.cheia = #avisos > 0 and (table.concat(avisos, "; ") ..
        string.format(" (%d itens, %d equipamentos)", mochila.itens, mochila.equipamentos)) or nil
end

local function ready()
    if not instance then
        local state = player.state()
        if not state or not state.id or state.id == 0 then return nil end
        -- Só a forte atira; as convidadas passam a vez na hora (pedido do usuário, 05/11).
        instance = require("instances.olimpiadas").new({party = party, attack_policy = "leader"})
        geracao = geracao + 1
    end
    local espera = AQUECIMENTO
    if player.state().id == party.leader_id then espera = AQUECIMENTO_LIDER end
    if session.now() < espera then return nil end
    return instance
end

-- Carteira: o servidor manda o saldo no pacote 38 (UPDATE_PRIVATE_INFO) sempre que ele muda.
-- O primeiro saldo visto nesta sessão é o ponto de partida; a diferença é o que entrou de fato,
-- venha de carta de ouro, baú ou qualquer outra fonte.
local carteira = {}

local function anotar_carteira(packet)
    if packet.code ~= 38 or packet.status ~= "decoded" or type(packet.fields) ~= "table" then return end
    for _, moeda in ipairs({"gold", "money"}) do
        local valor = packet.fields[moeda]
        if type(valor) == "number" then
            carteira[moeda .. "_inicio"] = carteira[moeda .. "_inicio"] or valor
            carteira[moeda] = valor
        end
    end
end

-- Lido pelo painel e pelo vigia do contêiner.
function grupo_status()
    return {mochila = mochila, total = totais(), recuperacoes = recuperacoes, captchas = captchas, carteira = carteira}
end

local proxima_verificacao = 0

return {
    api_version = 1,
    on_tick = function()
        local current = ready()
        if current then current.on_tick() end
        if instance and session.now() >= proxima_verificacao then
            verificar()
            proxima_verificacao = session.now() + 1
        end
    end,
    on_action = function()
        local current = ready()
        if current then current.on_action() end
    end,
    on_packet = function(packet)
        anotar_carteira(packet)
        local current = ready()
        if current then current.on_packet(packet) end
    end,
    on_close = function(event)
        if instance then instance.on_close(event) end
    end,
}
