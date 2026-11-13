-- Sondagem: descobre o número da instância. Cria a sala e manda room.setup com cada candidato,
-- anotando o que o servidor devolve no estado da sala. Não inicia partida.
local LIDER = 38878
SONDA = {fase = "inicio", i = 0, ids = {}, erros = {}, candidatos = {}}
for id = 1, 40 do SONDA.candidatos[#SONDA.candidatos + 1] = id end
local proximo = 0
local function ok(f, ...) local r = {pcall(f, ...)}; if not r[1] then table.insert(SONDA.erros, tostring(r[2])) end; return r[2] end
local function tick()
    if session.now() < proximo then return end
    proximo = session.now() + 2
    local me = player.state()
    if not me or me.id ~= LIDER or session.now() < 15 then return end
    local sala = room.state()
    if SONDA.fase == "inicio" and not sala.id then
        ok(scene.set_state, {state = 1}); ok(scene.login, {scene_id = 2})
        ok(room.create, {room_type = 4, time_type = 3, name = "Sonda", password = "sonda"})
        SONDA.fase = "criando"; return
    end
    if not sala.id then return end
    if SONDA.i > 0 then
        local c = SONDA.candidatos[SONDA.i]
        SONDA.ids[#SONDA.ids + 1] = string.format("%d->%s/%s", c, tostring(sala.map_id), tostring(sala.difficulty))
    end
    SONDA.i = SONDA.i + 1
    local c = SONDA.candidatos[SONDA.i]
    if not c then SONDA.fase = "fim"; return end
    ok(room.setup, {map_id = c, room_type = 4, password = "sonda", name = "Sonda",
        time_type = 1, difficulty = 1, level_limits = 0, flag_1 = false, flag_2 = false})
end
return {api_version = 1, on_tick = function() ok(tick) end, on_action = function() end,
    on_packet = function() end, on_close = function() end}
