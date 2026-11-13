-- Teste do estágio 1 da Olimpíadas (instância 6, Normal = 1): a líder cria a sala, inicia sozinha e, em cada
-- turno, atira UM tiro simples (sem itens) no balão de maior valor, registrando o que o servidor responde
-- (objetos removidos, placar, ações), para confirmar valor/cor dos balões e o raio de acerto.
local LIDER = 38878
local trajectory = require("trajectory")
local catalog = require("catalog")
T = {fase = "inicio", erros = {}, objetos = {}, valores = {}, log = {}, tiros = {}}
local proximo = 0
local function agora() return session.now() end
local function ok(f, ...) local r = {pcall(f, ...)}; if not r[1] then table.insert(T.erros, tostring(r[2])); T.erros[21] = nil end; return r[2] end
local function anota(s) table.insert(T.log, 1, string.format("%.1f %s", agora(), s)); T.log[121] = nil end

local function u32(h, o) return tonumber(h:sub(o, o + 7), 16) end
local function i32(h, o) local v = u32(h, o); if v >= 2147483648 then v = v - 4294967296 end; return v end
local function utf(h, o)
    local n = tonumber(h:sub(o, o + 3), 16); local s = {}
    for i = 0, n - 1 do s[#s + 1] = string.char(tonumber(h:sub(o + 4 + 2 * i, o + 5 + 2 * i), 16)) end
    return table.concat(s):gsub("%z", ""), o + 4 + 2 * n
end

local function no_pacote(p)
    if p.code ~= 91 or type(p.fields) ~= "table" or p.direction ~= "ServerToClient" then return end
    local f = p.fields; local sub = f.subcommand; local h = f.raw_hex
    if sub == 48 and h then
        local id, tipo, x, y = i32(h, 1), i32(h, 9), i32(h, 17), i32(h, 25)
        local modelo, o = utf(h, 33); local acao = utf(h, o)
        if modelo:find("ball", 1, true) then T.objetos[id] = {x = x, y = y, acao = acao, tipo = tipo} end
    elseif sub == 223 and h then
        local id = i32(h, 1); local _, o = utf(h, 9); local v = utf(h, o)
        local n = tonumber(v:match("shield(%-?%d+)"))
        if n then T.valores[id] = n end
        
    elseif sub == 53 and h then
        local id = i32(h, 1)
        
        T.objetos[id] = nil
    elseif sub == 104 then
        if T.atirou then anota("PONTOS " .. tostring(f.current_value_2) .. " rodada " .. tostring(f.current_value_1)) end
        T.pontos = f.current_value_2
    elseif T.atirou and (sub == 2 or sub == 11) then
        local partes = {}
        for k, v in pairs(f) do if type(v) ~= "table" then partes[#partes + 1] = k .. "=" .. tostring(v) end end
        anota(sub .. " " .. table.concat(partes, ","))
    end
end

local function caminho(modelo, tiro)
    local rad = math.rad(tiro.angle)
    local x, y = tiro.x, tiro.y
    local vx = tiro.force * math.cos(rad); vx = vx < 0 and math.ceil(vx) or math.floor(vx)
    local vy = tiro.force * math.sin(rad); vy = vy < 0 and math.ceil(vy) or math.floor(vy)
    local pts = {}
    for _ = 1, 400 do
        vx = vx + (modelo.wind_force - modelo.drag * vx) / modelo.mass * modelo.dt
        vy = vy + (modelo.gravity_force - modelo.drag * vy) / modelo.mass * modelo.dt
        x, y = x + vx * modelo.dt, y + vy * modelo.dt
        pts[#pts + 1] = {x, y}
        if y > 760 or x < 0 or x > 1800 then break end
    end
    return pts
end
local function pegos(pts, raio)
    local set, soma = {}, 0
    for id, o in pairs(T.objetos) do
        local v = T.valores[id]
        if v then
            for _, p in ipairs(pts) do
                if (p[1] - o.x) ^ 2 + (p[2] - o.y) ^ 2 <= raio * raio then set[#set + 1] = id .. ":" .. v; soma = soma + v; break end
            end
        end
    end
    return soma, set
end
local function mira(eu)
    local arma = catalog.weapon({id = eu.weapon_id})
    local proj = eu.ball_id or arma.common
    local vento = fight.wind().value or 0
    local modelo = trajectory.model({map_id = 1165, projectile_id = proj, wind = vento})
    local slope = trajectory.slope({map_id = 1165, x = eu.position.x, y = eu.position.y})
    local melhor
    for ang = math.ceil(arma.min_angle), math.floor(arma.max_angle), 2 do
        for forca = 400, 2000, 10 do
            local tiro = trajectory.launch({x = eu.position.x, y = eu.position.y, direction = 1, slope = slope, gun_angle = ang, force = forca})
            local soma = pegos(caminho(modelo, tiro), 35)
            if not melhor or soma > melhor.soma then melhor = {soma = soma, tiro = tiro, ang = ang, modelo = modelo} end
        end
    end
    return melhor, vento
end

local function tick()
    if agora() < proximo then return end
    proximo = agora() + 0.5
    local me = player.state()
    if not me or me.id ~= LIDER or agora() < 15 then return end
    local sala = room.state(); local luta = fight.state()
    if T.fase == "inicio" and not sala.id then
        ok(scene.set_state, {state = 1}); ok(scene.login, {scene_id = 2})
        ok(room.create, {room_type = 4, time_type = 3, name = "Teste", password = "teste"})
        T.fase = "criando"; return
    end
    if T.fase == "criando" and sala.id then
        ok(room.setup, {map_id = 6, room_type = 4, password = "teste", name = "Teste", time_type = 1,
            difficulty = 1, level_limits = 0, flag_1 = false, flag_2 = false})
        T.fase = "configurando"; T.em = agora(); return
    end
    if T.fase == "configurando" and agora() - T.em > 3 then ok(room.start); T.fase = "iniciando"; return end
    if not luta.map_id or luta.map_id == 0 then return end
    if luta.phase == "loading" and T.carregou ~= luta.generation then T.carregou = luta.generation; ok(fight.load_progress, {progress = 100}); return end
    local eu = fight.self(); local vez = luta.turn
    if not eu or not vez or vez.actor_id ~= eu.id or not eu.position or eu.destination then return end
    local chave = luta.generation .. ":" .. luta.turn_revision
    if T.vez == chave then return end
    local n = 0; for _ in pairs(T.objetos) do n = n + 1 end
    if n < 12 then return end
    T.vez = chave
    local m, vento = mira(eu)
    local pts = caminho(m.modelo, m.tiro)
    local prev = {}
    for _, r in ipairs({20, 30, 40, 50, 60}) do local so, se = pegos(pts, r); prev[#prev + 1] = r .. "=" .. so .. "{" .. table.concat(se, ",") .. "}" end
    anota(string.format("TIRO pontos_antes=%s ang=%d forca=%d vento=%s previsto %s", tostring(T.pontos), m.ang, m.tiro.force, tostring(vento), table.concat(prev, " ")))
    local tiro = m.tiro; local dir = 1
    T.atirou = true
    ok(fight.face, {direction = dir})
    ok(fight.shoot_tag, {started = true, shot_type = 2})
    ok(fight.shoot, tiro)
end

return {
    api_version = 1,
    on_tick = function() ok(tick) end,
    on_action = function() end,
    on_packet = function(p) if p then ok(no_pacote, p) end end,
    on_close = function() end,
}
