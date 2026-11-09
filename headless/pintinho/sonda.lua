-- Sondagem do Pintinho: só o líder cria a sala (instância 7, Difícil = 2), inicia e registra
-- o estado da sala, os mapas de cada estágio e os monstros. Não atira: passa a vez.
local LIDER = 80728
SONDA = {fase = "inicio", salas = {}, lutas = {}, pacotes = {}, erros = {}}
local proximo = 0
local function agora() return session.now() end
local function ok(f, ...) local r = {pcall(f, ...)}; if not r[1] then table.insert(SONDA.erros, tostring(r[2])) end; return r[2] end

local function tick()
    if agora() < proximo then return end
    proximo = agora() + 1
    local me = player.state()
    if not me or not me.id or me.id ~= LIDER or agora() < 15 then return end
    local sala = room.state()
    local luta = fight.state()
    if SONDA.fase == "inicio" and not sala.id then
        ok(scene.set_state, {state = 1}); ok(scene.login, {scene_id = 2})
        ok(room.create, {room_type = 4, time_type = 3, name = "Sonda", password = "sonda"})
        SONDA.fase = "criando"; return
    end
    if SONDA.fase == "criando" and sala.id then
        ok(room.setup, {map_id = SONDA.map_id or 7, room_type = 4, password = "sonda", name = "Sonda",
            time_type = 1, difficulty = SONDA.difficulty or 2, level_limits = 0, flag_1 = false, flag_2 = false})
        SONDA.fase = "configurando"; SONDA.setup_em = agora(); return
    end
    if SONDA.fase == "configurando" and agora() - SONDA.setup_em > 3 then
        SONDA.salas[#SONDA.salas + 1] = {map_id = sala.map_id, difficulty = sala.difficulty, players = #(sala.players or {})}
        if SONDA.auto_start then ok(room.start); SONDA.fase = "iniciando" else SONDA.fase = "pronto" end
        return
    end
    if luta and luta.map_id and luta.map_id ~= 0 then
        local chave = tostring(luta.generation) .. ":" .. tostring(luta.map_id)
        if not SONDA.lutas[chave] then
            SONDA.lutas[chave] = {map_id = luta.map_id, phase = luta.phase, t = agora()}
        end
        local l = SONDA.lutas[chave]
        l.phase = luta.phase
        l.state = luta
        local a = ok(fight.actors); if a then l.actors = a end
        if luta.phase == "loading" and not l.loaded then l.loaded = true; ok(fight.load_progress, {progress = 100}) end
        local t = ok(fight.turn)
        if SONDA.passar and t and t.actor_id == LIDER and t.revision ~= l.passou then l.passou = t.revision; ok(fight.pass) end
    end
end

return {
    api_version = 1,
    on_tick = function() ok(tick) end,
    on_action = function() end,
    on_packet = function(p)
        if p and p.code then
            local k = tostring(p.code)
            SONDA.pacotes[k] = (SONDA.pacotes[k] or 0) + 1
        end
    end,
    on_close = function() end,
}
