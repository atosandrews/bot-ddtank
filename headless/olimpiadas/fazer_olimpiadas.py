"""Gera olimpiadas/lua/instances/olimpiadas.lua a partir do motor do Pintinho (não altera o Pintinho).
Uso: python -I fazer_olimpiadas.py"""
from pathlib import Path

AQUI = Path(__file__).resolve().parent
SRC = AQUI.parent / "pintinho" / "lua" / "instances" / "pintinho.lua"
DST = AQUI / "lua" / "instances" / "olimpiadas.lua"
BALOES = (AQUI / "baloes.lua.txt").read_text(encoding="utf-8")
TURNO = (AQUI / "turno_baloes.lua.txt").read_text(encoding="utf-8")
s = SRC.read_text(encoding="utf-8")


def rep(old, new):
    global s
    n = s.count(old)
    assert n == 1, f"{n} ocorrências: {old[:90]!r}"
    s = s.replace(old, new)


head, tail = s.split("local M = {}", 1)
s = ("-- Olimpíadas Boom (instância 6, Normal = 1), derivado do motor do Pintinho (que veio do Formigueiro).\n"
     "-- Estágio 1 (1165): balões; o projétil \"pega\" todo balão a até 40 px da trajetória (medido em 12/11).\n"
     "-- Estágios 2 e 3 (1167/1168): chefes; só a líder ataca.\n"
     "local M = {}" + tail)

a = s.index("    local config = {")
b = s.index("    for key, value in pairs(options) do")
s = s[:a] + (AQUI / "config.lua.txt").read_text(encoding="utf-8") + "\n" + s[b:]

rep('"Unknown Pintinho option: "', '"Unknown Olimpiadas option: "')
rep('state.reason = "Waiting for Pintinho / Difícil confirmation"', 'state.reason = "Waiting for Olimpiadas / Normal confirmation"')
rep('pause("Server room configuration does not match Pintinho / Difícil")', 'pause("Server room configuration does not match Olimpiadas / Normal")')

rep('local known = {extra_attack_2 = extra_attack_2, max_damage = max_damage}',
    'local known = {extra_attack_2 = extra_attack_2, extra_attack = extra_attack, max_damage = max_damage}')
rep("        for _, prop in ipairs(damage_props) do\n            cost = tonumber(prop.properties[4])",
    "        if config.no_damage_maps[tostring(fight_state.map_id)] then return plan end\n"
    "        for _, prop in ipairs(damage_props) do\n            cost = tonumber(prop.properties[4])")

rep("            shot_log = {},\n        }",
    "            shot_log = {},\n            balloons = {},\n            balloon_values = {},\n            balloon_picked = {},\n            balloon_log = {},\n            hp_log = {},\n        }")

rep("    local function attack(fight_state)\n", BALOES + "\n    local function attack(fight_state)\n")

rep('        if config.attack_policy == "leader" and not party:is_leader() then',
    '        if config.attack_policy == "leader" and not party:is_leader()\n'
    '            and not config.all_attack_maps[tostring(fight_state.map_id)] then')

rep("""                and (now() - state.followup_started) >= config.followup_delay then
                send(fight.shoot, state.shot)""", """                and (now() - state.followup_started) >= config.followup_delay then
                if config.balloon_maps[tostring(fight_state.map_id)] then
                    -- balões: cada tiro extra recalcula a mira sem os balões já pegos (pedido do usuário)
                    local again = balloon_shot(self, fight_state, state.balloon_trident)
                    if again and again.score > 0 then
                        for _, id in ipairs(again.ids) do state.balloon_picked[id] = true end
                        state.shot = again.shot
                        state.balloon_note = (state.balloon_note or "") .. string.format(" | extra %.1f [%s]", again.score, table.concat(again.ids, ","))
                    end
                end
                send(fight.shoot, state.shot)""")

rep("""        local fly = config.fly_maps[tostring(fight_state.map_id)]
        local fly_key = tostring(fight_state.map_id)""", TURNO + """
        local fly = config.fly_maps[tostring(fight_state.map_id)]
        local fly_key = tostring(fight_state.map_id)""")

rep("""    local function on_packet(packet)
        if state.phase == "incomplete" or state.phase == "paused" then return end""", """    local function on_packet(packet)
        if packet then pcall(observe_balloons, packet); pcall(observe_hp) end
        if state.phase == "incomplete" or state.phase == "paused" then return end""")

rep("function pintinho_stop_after_run()", "function olimpiadas_stop_after_run()")
rep("function pintinho_status()", "function olimpiadas_status()")
rep("my_turns = state.my_turns, fly_note", "balloon_note = state.balloon_note, balloon_log = state.balloon_log, hp_log = state.hp_log, shot_log = state.shot_log, balloons = #balloon_list(), my_turns = state.my_turns, fly_note")

# Estágio 3 (pedido do usuário): bater na cabeça ou nas costas do Grupo de Fans. Ponto de mira deslocado para trás
# (do lado oposto a quem atira) e para cima, com ângulos altos (o tiro desce por cima).
rep("""        local destination = {
            x = target.position.x,
            y = math.max(0, target.position.y - config.aim_height),
            radius = config.aim_radius,
        }""", """        local destination = {
            x = target.position.x,
            y = math.max(0, target.position.y - config.aim_height),
            radius = config.aim_radius,
        }
        local offset = config.aim_offset_maps[tostring(fight_state.map_id)]
        if offset and target.id ~= -1 then
            -- 13/11: "costas" = lado oposto para onde o chefe está virado (Grupo de Fans "(Direito)"), não o lado oposto a
            -- quem atira: pela frente a Mesa não morria (2 derrotas); pelas costas morreu em todas as vitórias.
            local facing
            if offset.back then
                for _, actor in ipairs(fight_state.actors) do
                    if name_rank(actor.name, {offset.back}) and actor.direction and actor.direction ~= 0 then facing = actor.direction end
                end
            end
            if facing then
                destination.x = target.position.x - facing * offset.dx
                state.back_note = string.format("%s: chefe virado para %d, mira x=%d", tostring(target.name), facing, destination.x)
            else
                destination.x = destination.x + direction * offset.dx
            end
            destination.y = math.max(0, destination.y - offset.dy)
        end""")
rep("        for _, preferred in ipairs(rotated(config.angles, stuck_shift(fight_state))) do",
    "        for _, preferred in ipairs(rotated((offset and offset.angles) or config.angles, stuck_shift(fight_state))) do")

# Estágio 2 (12/11): nascendo colada no boxeador ela se explode e ele não toma dano: voa para longe só nesse caso.
rep("""            if not house or energy < 150 or not controls.known""", """            if house and fly.only_within and math.abs(house.x - self.position.x) > fly.only_within then
                state.flown[fly_key] = "not_needed"
                state.fly_note = string.format("no fly: %d px from %s", math.abs(house.x - self.position.x), fly.target)
            elseif not house or energy < 150 or not controls.known""")

# Registro dos tiros com a vida dela (12/11: separar dano da própria explosão e dano do chefe).
rep("""            table.insert(state.shot_log, 1, string.format("[t%d s%d %s%s alvo=%s eu@%s,%s f=%s""", """            table.insert(state.shot_log, 1, string.format("[t%d s%d %s%s alvo=%s eu@%s,%s hp=%s f=%s""")
rep("""                tostring(target.id), tostring(self.position.x), tostring(self.position.y),
                tostring(solution.shot.force)""", """                tostring(target.id), tostring(self.position.x), tostring(self.position.y), tostring(self.hp),
                tostring(solution.shot.force)""")

# Chefes perto (regra do usuário, 12/11): afasta um pouco e bate na cabeça.
PASSO = (AQUI / "passo_chefe.lua.txt").read_text(encoding="utf-8")
rep("        local candidates = target_candidates(self, fight_state)\n", PASSO + "        local candidates = target_candidates(self, fight_state)\n")
s = s.replace("""        local offset = config.aim_offset_maps[tostring(fight_state.map_id)]""",
    """        -- 13/11: a Mesa sempre com a regra dela (tiro alto), mesmo perto do chefe; perto, o chefe leva tiro na cabeça;
        -- sem caminho para o tiro deslocado (state.no_offset), mira direto no corpo.
        local offset
        if not state.no_offset then
            local close_rule = config.close_step[tostring(fight_state.map_id)]
            local entries = config.aim_offset_maps[tostring(fight_state.map_id)]
            if entries and not entries[1] then entries = {entries} end
            local own
            for _, entry in ipairs(entries or {}) do
                if not own and (not entry.only or name_rank(target.name, {entry.only})) then own = entry end
            end
            local head = state.close_head and close_rule and close_rule.head
            if head and head.only and not name_rank(target.name, {head.only}) then head = nil end
            if own and own.only and head and name_rank(own.only, {"Mesa"}) then offset = own
            else offset = head or own end
        end""")
rep("            minimum_clearance, tostring(use_trident == true), tostring(allow_risk == true), stuck_shift(fight_state),",
    "            minimum_clearance, tostring(use_trident == true), tostring(allow_risk == true), stuck_shift(fight_state),\n"
    "            tostring(state.no_offset == true), tostring(state.close_head == true),")
rep("""        if not solution then
            error({code = "formigueiro_no_shot", message = "No trajectory could be calculated from the server position"}, 0)
        end""", """        -- 13/11: o tiro por cima/por trás sem caminho virava "melhor esforço" com força 1 (não fazia nada): tenta direto no corpo.
        if solution and not solution.direct and config.aim_offset_maps[tostring(fight_state.map_id)] then
            state.no_offset = true
            for _, candidate in ipairs(candidates) do
                local found, result = pcall(aim, self, candidate, fight_state, nil, state.trident_requested or full_trident, true)
                if found and result.direct then
                    target, solution = candidate, result
                    state.offset_note = "no path for the offset aim; direct at " .. tostring(candidate.name)
                    break
                elseif not found and (type(result) ~= "table" or result.code ~= "formigueiro_no_shot") then
                    state.no_offset = false
                    error(result, 0)
                end
            end
            state.no_offset = false
        end
        if not solution then
            error({code = "formigueiro_no_shot", message = "No trajectory could be calculated from the server position"}, 0)
        end""")

# Passo para longe do chefe recusado pelo servidor: repete (até 3x) em vez de pausar.
rep("""                if state.walk_retries <= 3 then state.walk_sent = nil end""", """                if state.walk_retries <= 3 then state.walk_sent = nil; state.step_key = nil end
                state.step_sent = nil""")

DST.parent.mkdir(parents=True, exist_ok=True)
DST.write_text(s, encoding="utf-8")
print("ok", len(s.splitlines()), "linhas")
