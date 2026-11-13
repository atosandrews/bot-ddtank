-- Olimpíadas Boom (instância 6, Normal = 1), derivado do motor do Pintinho (que veio do Formigueiro).
-- Estágio 1 (1165): balões; o projétil "pega" todo balão a até 40 px da trajetória (medido em 12/11).
-- Estágios 2 e 3 (1167/1168): chefes; só a líder ataca.
local M = {}

function M.new(options)
    options = options or {}
    local catalog = require("catalog")
    local terrain = require("terrain")
    local trajectory = require("trajectory")
    local rewards = require("rewards")
    local damage_props = {
        require("data.items.resources.danos_acrescentados_em_50_10004"),
        require("data.items.resources.danos_acrescentados_em_40_10005"),
        require("data.items.resources.danos_acrescentados_em_30_10006"),
        require("data.items.resources.danos_acrescentados_em_20_10007"),
        require("data.items.resources.danos_acrescentados_em_10_10008"),
    }
    local trident = require("data.items.resources.missil_tridente_10003")
    local extra_attack = require("data.items.resources.ataque_de_1_vez_complementar_10002")
    local extra_attack_2 = require("data.items.resources.ataque_de_2_vezes_complementares_10001")
    local max_damage = require("data.items.resources.danos_acrescentados_ao_maximo_10000")

    require("data.projectiles")
    require("data.weapons")

    local config = {
        room_name = "Olimpiadas",
        password = "olimp",
        map_id = 6,
        difficulty = 1,
        stage_maps = {1165, 1167, 1168},
        stage_order = {1165, 1167, 1168},
        trident_maps = {},
        cluster_maps = {},
        -- Pedido do usuário (12/11): estágio 1 = tridente + 2 (as 4 contas); estágio 2 = +1 + tridente + dano
        -- máximo (só a líder); estágio 3 = todo o dano (+2 e a sobra de energia em dano; só a líder).
        stage_extra_props = {
            ["1165"] = {from_turn = 1, trident = true, "extra_attack_2"},
            -- 12/11: sem tridente o boxeador tomava 0-1,3 mil por turno; com +1 + tridente + dano, ~7 mil (regra original).
            ["1167"] = {from_turn = 1, trident = true, "extra_attack"},
            ["1168"] = {from_turn = 1, trident = false, "extra_attack_2"},
        },
        -- Balões: sem dano extra (não muda os pontos).
        no_damage_maps = {["1165"] = true},
        -- Mapas em que todas as contas atacam (no resto, só a líder).
        all_attack_maps = {["1165"] = true},
        -- Balões (estágio 1): raio de "pega" em volta da trajetória (40 medido; 38 por margem). Usuário (12/11): pode pegar
        -- verde se o total do tiro for maior, então o verde conta só o valor dele (peso 1). Peso dos verdes
        -- (negativos) e valor do florido (0 no pacote).
        balloon_maps = {["1165"] = {radius = 38, negative_weight = 1, zero_value = 1, ground_y = 720}},
        -- Estágio 3: a vitória vem ao matar a "Mesa dos Juízes" (1500, aparece por um turno no lugar do chefe), não o
        -- Grupo de Fans (vitórias com ele em 25 mil, 15 mil, 9 mil...). Mesa primeiro.
        target_first = {"Mesa dos Ju", "Meka 6132", "Grupo de Fans"},
        target_last = {},
        -- Estágio 2: só o boxeador (o campeão, 18 mil); estágio 3: só o Grupo de Fans.
        target_only = {["1167"] = "Meka 6132"},
        -- Estágio 2 (pedido do usuário, 12/11): a líder fica o mais perto possível do boxeador: voa no 1º turno.
        -- 12/11: com 60 px ela pousou colada (x=1590, boxeador em 1650) e morreu com os próprios tiros (18 mil intactos).
        -- 12/11: o voo (60 e 260 px) a deixou em cima do boxeador e ela morreu em 3 turnos com 18 mil intactos: sem voo
        -- até entender a distância certa com o usuário.
        -- 12/11: ela nasce em lugares diferentes; colada no boxeador (x=1600, ele em 1650) se explode e ele fica intacto.
        -- Só quando nascer a menos de "only_within" px, voa para o lado oposto, a "standoff" px dele (de 650 px, 3 turnos).
        -- (o voo para longe não adiantou: no turno seguinte ela estava colada de novo; agora vale o close_step)
        fly_maps = {},
        fly_prop = 10016,
        -- Chefe perto (usuário, 12/11: "afastar um pouco e bater na cabeça"): a menos de "radius" px anda até "max_walk"
        -- px para longe (2 de energia por px, guardando "keep_energy" para os itens) e mira na cabeça, com tiro alto.
        close_step = {
            ["1167"] = {target = "Meka 6132", radius = 150, max_walk = 70, keep_energy = 220, settle = 1.2,
                head = {only = "Meka 6132", dx = 0, dy = 70, angles = {80, 75, 70, 65, 60}}},
            ["1168"] = {target = "Grupo de Fans", skip_when = "Mesa dos Ju", radius = 150, max_walk = 70, keep_energy = 195, settle = 1.2,
                head = {only = "Grupo de Fans", dx = 0, dy = 70, angles = {80, 75, 70, 65, 60}}},
        },
        -- Estágio 3 (usuário: "dano em cima da cabeça dele ou nas costas"): de frente o tiro tira 1-3 mil; de costas
        -- tirou 12 mil (12/11). Mira dx px atrás e dy px acima do ponto do servidor, ângulos altos primeiro.
        -- Mesa dos Juízes: tiro alto caindo em cima dela (12/11: de plataforma, tiro baixo de 25° bateu no chefe 3x e ela
        -- tomou 2 golpes de ~7 mil do chefe com a Mesa viva).
        aim_offset_maps = {["1168"] = {
            {only = "Mesa dos Ju", back = "Grupo de Fans", dx = 30, dy = 0, angles = {65, 70, 60, 75, 55, 50}},
            {only = "Grupo de Fans", back = "Grupo de Fans", dx = 35, dy = 60, angles = {70, 65, 75, 60, 55, 50}},
        }},
        walk_back = {},
        -- Chefes: itens sempre, mesmo sem tiro limpo (antes atirava sem itens no "close_shot").
        full_maps = {["1167"] = 1, ["1168"] = 1},
        -- Só para registrar os tiros do estágio 2 na nota de derrota (o giro de mira só começaria no turno 1000).
        stuck_turns = {["1167"] = 1000, ["1168"] = 1000},
        close_kill = {},
        fly_projectile = 3,
        aim_height = 18,
        aim_radius = 12,
        max_steps = 300,
        angles = {25, 35, 45, 60, 75},
        buff_timeout = 3,
        fallback_wind = 0,
        final_cards = 3,
        extra_card_timeout = 3,
        extra_retry_runs = 10,
        followup_delay = 0.3,
        retry_delay = 0.1,
        room_retry_limit = 5,
        room_retry_max_delay = 1,
        start_timeout = 15,
        single_shot_clearance = 120,
        self_clearance = 180,
        nearby_actor_radius = 40,
        attack_policy = "all",
    }

    for key, value in pairs(options) do
        if key ~= "party" then
            assert(config[key] ~= nil, "Unknown Olimpiadas option: " .. key)
            config[key] = value
        end
    end

    local supported_maps = {}
    for _, map_id in ipairs(config.stage_maps) do
        terrain.solid({map_id = map_id, x = 0, y = 0})
        supported_maps[tostring(map_id)] = true
    end

    local function name_rank(name, list)
        name = string.lower(tostring(name or ""))
        for index, pattern in ipairs(list) do
            if string.find(name, string.lower(pattern), 1, true) then return index end
        end
        return nil
    end

    local function new_run()
        return {
            phase = "home",
            reason = "Waiting for server state",
            pending = {},
            loaded = {},
            turn_key = "",
            fired_turn = "",
            card_revision = -1,
            cards_selected = 0,
            card_waiting = false,
            card_finished = false,
            continued = false,
            configured = false,
            setup_rechecks = 0,
            room_recoveries = 0,
            aim_cache = {},
            aim_jobs = {},
            collection = rewards.new(),
            preflight = rewards.new(),
            stage_count = 0,
            stages_seen = {},
            flown = {},
            walked = {},
            my_turns = {},
            shot_log = {},
            balloons = {},
            balloon_values = {},
            balloon_picked = {},
            balloon_log = {},
            hp_log = {},
        }
    end

    -- Cronômetro: segundos do início da vez até o primeiro tiro e entre os tiros extras (últimos 40).
    local reacoes = {}
    local derrotas = {}  -- últimas 10 derrotas: mapa, vida e turnos no momento

    local function cronometrar(texto)
        table.insert(reacoes, 1, texto)
        reacoes[41] = nil
    end

    local party
    if options.party then party = require("instances.party").new(options.party, config) end
    if config.attack_policy ~= "all" and config.attack_policy ~= "leader" then
        error({code = "formigueiro_config", message = "Attack policy must be all or leader"}, 0)
    end
    if config.attack_policy == "leader" and not party then
        error({code = "formigueiro_config", message = "Leader attack policy requires a party"}, 0)
    end

    local state = new_run()

    local function stage_plan(map_id)
        local plan = config.stage_extra_props[tostring(map_id)]
        if plan and (state.my_turns[tostring(map_id)] or 0) >= (plan.from_turn or 1) then return plan end
        return nil
    end

    local function uses_trident(map_id)
        if config.trident_maps[tostring(map_id)] then return true end
        local plan = stage_plan(map_id)
        -- Com recuo configurado, tridente só depois de andar: parada no pouso, o míssil curto a acerta (~2000 por turno).
        -- 08/11 (regra do usuário): do turno "full_maps" em diante, tridente sempre, tenha andado ou não.
        local full_from = config.full_maps[tostring(map_id)]
        local full = full_from and (state.my_turns[tostring(map_id)] or 0) >= full_from
        if not full and config.walk_back[tostring(map_id)] and state.walked[tostring(map_id)] ~= true then return false end
        return plan ~= nil and plan.trident ~= false
    end

    state.collection_map = fight.state().map_id
    local farm = {completed_runs = 0, victories = 0, defeats = 0, deliveries = {}}

    local function now()
        return session.now()
    end

    local blocked_phases = {
        paused = true, incomplete = true, configuration_unconfirmed = true,
        start_unconfirmed = true, start_cancelled = true,
    }

    local function report_status()
        if not session.report then return end

        local status = "running"
        if state.phase == "stopped" then
            status = "completed"
        elseif blocked_phases[state.phase] then
            status = "paused"
        elseif state.phase == "verification" or #state.pending > 0 or string.sub(state.reason or "", 1, 7) == "Waiting" then
            status = "waiting"
        end

        local reason = state.reason or ""
        if #reason > 1024 then reason = "Status detail exceeds the observation limit; inspect pintinho_status()" end
        session.report({status = status, phase = state.phase, reason = reason})
    end

    local function pause(reason)
        state.phase = "paused"
        state.reason = reason
    end

    local function send(command, arguments)
        local id
        if arguments then
            id = command(arguments)
        else
            id = command()
        end

        state.pending[#state.pending + 1] = id
        state.last_action = id
    end

    local function skip_buff(reason)
        local waiting = state.buff_waiting
        state.buff_issue = reason
        state.props_done = true
        if not waiting then return end
        if waiting.kind == "POW" then
            state.pow = "unconfirmed"
        else
            state.damage_reason = reason
        end
        state.buff_waiting = nil
    end

    local function writes_complete()
        local receipts = {}
        local pending = false
        for _, id in ipairs(state.pending) do
            local receipt = actions.get(id)
            if not receipt then
                pause("Command receipt is unavailable: " .. tostring(id))
                return false
            end
            if receipt.status == "uncertain" then
                pause(receipt.name .. ": uncertain: " .. (receipt.reason or "No reason supplied"))
                return false
            end
            if receipt.status == "queued" or receipt.status == "dispatched" then pending = true end
            receipts[#receipts + 1] = receipt
        end
        if pending then return false end

        if #receipts == 3 and receipts[1].name == "scene_state"
            and receipts[2].name == "scene_login" and receipts[3].name == "room_create" then
            local unwritten = true
            for _, receipt in ipairs(receipts) do
                local stale = receipt.reason == "stale_state" or receipt.reason == "stale_binding"
                if receipt.status ~= "rejected" or not stale then unwritten = false end
            end
            if unwritten then
                state.pending = {}
                state.phase = "home"
                state.retry_at = now() + config.retry_delay
                state.reason = "Central and room creation were not written; recheck after the retry delay"
                return false
            end
        end

        if state.shot_action and #receipts > 0 then
            local shot = actions.get(state.shot_action)
            local stale = false
            for _, receipt in ipairs(receipts) do
                if receipt.status == "rejected" and receipt.reason == "stale_state" then stale = true end
            end
            if stale and (shot.status == "rejected" or shot.status == "cancelled") then
                state.pending = {}
                state.shot_action = nil
                if state.shot_followup then
                    state.followup_sent = false
                    state.followups = math.max(0, (state.followups or 1) - 1)
                    state.followup_started = now()
                else
                    state.fired_turn = ""
                end
                state.retry_at = now() + config.retry_delay
                state.reason = "Shot was not written; recheck current state after the retry delay"
                return false
            end
        end

        for _, id in ipairs(state.pending) do
            local result = actions.get(id)
            if not result then
                pause("Command receipt is unavailable: " .. tostring(id))
                return false
            end

            -- Voo recusado (normalmente stale_state): até 3 tentativas no turno; depois ataca sem voar e tenta no próximo.
            if #state.pending == 1 and state.fly_action and id == state.fly_action
                and (result.status == "rejected" or result.status == "cancelled") then
                state.fly_retries = (state.fly_retries or 0) + 1
                state.fly_note = (state.fly_note or "") .. " [fly " .. result.status .. ":" .. tostring(result.reason) .. "]"
                if state.fly_retries <= 3 then state.fly_prop_sent = nil else state.fly_prop_sent = "failed" end
                state.fly_action = nil
                state.pending = {}
                state.retry_at = now() + config.retry_delay
                state.reason = "Fly prop was not written; retrying"
                return false
            end

            -- Andar recusado (normalmente stale_state: chegou um evento no mesmo instante): tenta de novo até 3 vezes,
            -- sem cancelar os adereços do turno.
            if #state.pending == 1 and state.walk_action and id == state.walk_action
                and (result.status == "rejected" or result.status == "cancelled") then
                state.walk_retries = (state.walk_retries or 0) + 1
                state.walk_note = (state.walk_note or "") .. " [" .. result.status .. ":" .. tostring(result.reason) .. "]"
                if state.walk_retries <= 3 then state.walk_sent = nil; state.step_key = nil end
                state.step_sent = nil
                state.walk_action = nil
                state.pending = {}
                state.retry_at = now() + config.retry_delay
                state.reason = "Walk was not written; retrying"
                return false
            end

            if #state.pending == 1 and result.status == "rejected" and result.reason == "stale_state" then
                local recoverable = false
                if result.name == "card_take" or result.name == "card_take_boss" or result.name == "card_take_extra" then
                    state.card_waiting = false
                    state.card_waiting_index = nil
                    recoverable = true
                elseif result.name == "card_finish" or result.name == "card_finish_boss" then
                    state.card_finished = false
                    recoverable = true
                elseif result.name == "mission_start" then
                    state.continued = false
                    recoverable = true
                elseif result.name == "skip_turn" then
                    state.passed_turn = nil
                    recoverable = true
                end

                if recoverable then
                    state.pending = {}
                    state.retry_at = now() + config.retry_delay
                    state.reason = "Command " .. result.name .. " was not written; recheck current state after the retry delay"
                    return false
                end
            end

            if #state.pending == 1 and result.status == "rejected" and result.reason == "stale_state"
                and result.name == "load_progress" then
                local current = fight.state()
                if current.generation == state.loading_generation and current.map_id == state.loading_map then
                    if current.phase == "active" then
                        state.pending = {}
                        return true
                    end
                    if current.phase == "loading" then
                        state.loaded[state.loading_generation .. ":" .. state.loading_map] = nil
                        state.pending = {}
                        state.retry_at = now() + config.retry_delay
                        state.reason = "Loading progress was not written; recheck the stage after the retry delay"
                        return false
                    end
                end
            end

            if #state.pending == 1 and result.status == "rejected" and result.reason == "stale_state"
                and (result.name == "room_setup" or result.name == "room_start") and state.setup_rechecks < config.room_retry_limit then
                state.setup_rechecks = state.setup_rechecks + 1
                local delay = math.min(config.room_retry_max_delay, config.retry_delay * 2 ^ (state.setup_rechecks - 1))
                state.retry_at = now() + delay
                state.pending = {}
                if result.name == "room_setup" then
                    state.phase = "creating"
                    state.configured = false
                else
                    state.phase = "configuring"
                end
                state.reason = "Room command was not written; waiting to recheck server state"
                return false
            end

            if #state.pending == 1 and id == state.buff_action and state.buff_waiting
                and result.status == "rejected" and result.reason == "stale_state" and (state.buff_rechecks or 0) < 2 then
                state.buff_rechecks = (state.buff_rechecks or 0) + 1
                if state.buff_waiting.kind == "POW" then state.pow = nil
                else state.prop_index = state.prop_index - 1 end
                state.buff_waiting = nil
                state.buff_action = nil
                state.pending = {}
                state.retry_at = now() + config.retry_delay
                state.reason = "Buff was not written; recheck current state after the retry delay"
                return false
            end

            if #state.pending == 1 and id == state.buff_action
                and (result.status == "rejected" or result.status == "cancelled") then
                skip_buff("Optional buff was not sent: " .. result.status .. " (" .. tostring(result.reason) .. ")")
                state.pending = {}
                state.buff_action = nil
                return true
            end

            if result.status == "rejected" or result.status == "uncertain" or result.status == "cancelled" then
                pause(result.name .. ": " .. result.status .. ": " .. (result.reason or "No reason supplied"))
                return false
            end

            if result.status ~= "sent" then
                return false
            end
            if result.name == "room_setup" or result.name == "room_start" then
                state.setup_rechecks = 0
            end
        end

        state.pending = {}
        state.shot_action = nil
        state.buff_action = nil
        state.walk_action = nil
        state.fly_action = nil
        return true
    end

    local function solo(room_state, own_id)
        if party then return party:check_members(room_state) end
        for _, member in ipairs(room_state.players) do
            if member.id ~= own_id then
                pause("Another player is in the room; this script requires a solo room")
                return false
            end
        end

        return true
    end

    local function prepare_room(room_state)
        if party then
            if not party:settle() then return end
            if not state.preflight_done then
                local collected, reason = state.preflight:step({})
                state.reason = reason
                if not collected then return end
                state.preflight_done = true
                state.collection = rewards.new()
            end
            if not party:is_leader() then
                state.reason = party:prepare_follower(room_state)
                return
            end
        end
        if state.phase == "home" and not room_state.id then
            send(scene.set_state, {state = 1})
            send(scene.login, {scene_id = 2})
            send(room.create, {
                room_type = 4, time_type = 3, name = config.room_name, password = config.password,
            })
            state.phase = "creating"
            state.reason = "Waiting for room creation"
            return
        end

        if not room_state.id or #room_state.players == 0 then
            return
        end

        if state.phase == "home" or state.phase == "creating" then
            if room_state.started and not state.restart_generation then
                pause("The room has already started; wait for its fight state before loading the script")
                return
            end

            state.setup_requested_at = now()
            send(room.setup, {
                map_id = config.map_id, room_type = 4, password = config.password, name = config.room_name,
                time_type = 1, difficulty = config.difficulty, level_limits = 0, flag_1 = false, flag_2 = false,
            })
            state.room_id = room_state.id
            state.phase = "configuring"
            state.reason = "Waiting for Olimpiadas / Normal confirmation"
            return
        end

        if state.phase == "configuring" and not state.configured and state.setup_requested_at
            and now() - state.setup_requested_at >= config.start_timeout then
            state.phase = "configuration_unconfirmed"
            state.reason = "No server room configuration within 15 seconds; the sent request was not retried"
            return
        end

        if state.phase == "configuring" and state.configured then
            if room_state.map_id ~= config.map_id or room_state.difficulty ~= config.difficulty then
                pause("Server room configuration does not match Olimpiadas / Normal")
                return
            end

            if party then
                local ready, reason = party:prepare_leader(room_state)
                state.reason = reason
                if not ready then return end
            end

            state.start_cancelled = nil
            state.start_cancelled_revision = room_state.start_cancelled_revision
            state.start_requested_at = now()
            send(room.start)
            state.phase = "starting"
            state.reason = "Waiting for the fight"
        end
    end

    local function trunc(value)
        if value < 0 then return math.ceil(value) end
        return math.floor(value)
    end

    local function height_at_target(model, shot, target, direction)
        local radians = math.rad(shot.angle)
        local point = {
            x = shot.x, y = shot.y,
            vx = trunc(shot.force * math.cos(radians)), vy = trunc(shot.force * math.sin(radians)),
        }

        for _ = 1, config.max_steps do
            local next_point = trajectory.step(point, model)
            if (next_point.x - target.x) * direction >= 0 and (point.x - target.x) * direction <= 0 then
                local dx = next_point.x - point.x
                if dx == 0 then return point.y - target.y end
                local fraction = (target.x - point.x) / dx
                return point.y + fraction * (next_point.y - point.y) - target.y
            end

            if next_point.y > model.height or next_point.vx * direction <= 0 then
                return math.huge
            end

            point = next_point
        end

        return math.huge
    end

    local function segment_distance_squared(x, y, from, to)
        local dx, dy = to.x - from.x, to.y - from.y
        local length = dx * dx + dy * dy
        local fraction = 0
        if length > 0 then
            fraction = math.max(0, math.min(1, ((x - from.x) * dx + (y - from.y) * dy) / length))
        end
        return (from.x + fraction * dx - x) ^ 2 + (from.y + fraction * dy - y) ^ 2
    end

    local function clears_player(model, shot, self, nearby, minimum_clearance)
        local radians = math.rad(shot.angle)
        local point = {x = shot.x, y = shot.y, vx = trunc(shot.force * math.cos(radians)), vy = trunc(shot.force * math.sin(radians))}
        local clearance = minimum_clearance ^ 2
        for _ = 1, config.max_steps do
            local next_point = trajectory.step(point, model)
            local contact = terrain.trace({map_id = model.map_id, x = point.x, y = point.y, to_x = next_point.x, to_y = next_point.y})
            for _, actor in ipairs(nearby) do
                if segment_distance_squared(actor.x, actor.y, point, contact) <= config.nearby_actor_radius ^ 2 then
                    return false
                end
            end
            if contact.hit then
                return (contact.x - self.position.x) ^ 2 + (contact.y - self.position.y) ^ 2 >= clearance
            end
            if next_point.x < 0 or next_point.x > model.width or next_point.y > model.height then return true end
            point = next_point
        end
        return false
    end

    local function better_shot(candidate, current)
        if not current then return true end
        if candidate.direct ~= current.direct then return candidate.direct end
        return candidate.distance < current.distance
    end

    local function aim_compute(cache, key, calculate)
        if cache[key] ~= nil then return cache[key] end
        if now() >= state.search_deadline then
            error({code = "formigueiro_aim_pending", message = "Continuing the bounded aim search"}, 0)
        end
        local result = calculate()
        cache[key] = result
        return result
    end

    local function shot_key(shot)
        return table.concat({shot.x, shot.y, shot.angle, shot.force}, ":")
    end

    -- 0 normalmente; 1, 2, 3... a cada turno dela depois de "stuck_turns" no estágio (ver config.stuck_turns).
    local function stuck_shift(fight_state)
        local map = tostring(fight_state.map_id)
        local limit = config.stuck_turns[map]
        local turns = state.my_turns[map] or 0
        if limit and turns >= limit then return turns - limit + 1 end
        return 0
    end

    local function rotated(list, shift)
        if shift == 0 or #list < 2 then return list end
        local out = {}
        for i = 1, #list do out[i] = list[((i - 1 + shift) % #list) + 1] end
        return out
    end

    local function aim(self, target, fight_state, projectile_override, use_trident, allow_risk)
        local weapon = catalog.weapon({id = self.weapon_id})
        local projectile = projectile_override or self.ball_id or weapon.common
        local direction = 1
        if target.position.x < self.position.x then direction = -1 end

        local destination = {
            x = target.position.x,
            y = math.max(0, target.position.y - config.aim_height),
            radius = config.aim_radius,
        }
        -- 13/11: a Mesa sempre com a regra dela (tiro alto), mesmo perto do chefe; perto, o chefe leva tiro na cabeça;
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
        end
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
        end
        local minimum_clearance = config.single_shot_clearance
        if use_trident or self.special or projectile_override then minimum_clearance = config.self_clearance end
        if not allow_risk and (destination.x - self.position.x) ^ 2 + (destination.y - self.position.y) ^ 2 < minimum_clearance ^ 2 then
            error({code = "formigueiro_no_shot", message = "Target is too close for this projectile"}, 0)
        end

        local observed_wind = fight.wind()
        local wind = config.fallback_wind
        state.wind_fallback = true
        if observed_wind.current and type(observed_wind.value) == "number" then
            wind = observed_wind.value
            state.wind_fallback = false
        end
        state.wind_used = wind
        local nearby = {}
        local inputs = {
            fight_state.generation, fight_state.turn_revision, fight_state.map_id, self.weapon_id, projectile,
            self.position.x, self.position.y, target.id, destination.x, destination.y, wind,
            minimum_clearance, tostring(use_trident == true), tostring(allow_risk == true), stuck_shift(fight_state),
            tostring(state.no_offset == true), tostring(state.close_head == true),
        }
        for _, actor in ipairs(fight_state.actors) do
            local hp = actor.pending_hp
            if hp == nil then hp = actor.hp end
            local position = actor.destination or actor.position
            if actor.id ~= self.id and actor.alive ~= false and hp and hp > 0 and position
                and (position.x - self.position.x) ^ 2 + (position.y - self.position.y) ^ 2
                    < minimum_clearance ^ 2 then
                nearby[#nearby + 1] = {x = position.x, y = position.y - config.aim_height}
                inputs[#inputs + 1] = actor.id
                inputs[#inputs + 1] = position.x
                inputs[#inputs + 1] = position.y
            end
        end

        local cache_key = table.concat(inputs, ":")
        if state.aim_cache[cache_key] then return state.aim_cache[cache_key] end
        local job = state.aim_jobs[cache_key]
        if not job then
            job = {heights = {}, predictions = {}, clearance = {}}
            state.aim_jobs[cache_key] = job
        end
        if job.failed then
            error({code = "formigueiro_no_shot", message = "No clear shot found for target " .. target.id}, 0)
        end
        local model = trajectory.model({map_id = fight_state.map_id, projectile_id = projectile, wind = wind})
        local slope = trajectory.slope({map_id = fight_state.map_id, x = self.position.x, y = self.position.y})
        local tried = {}
        local closest
        local function predict_uncached(shot)
            local result = trajectory.simulate({
                map_id = fight_state.map_id, projectile_id = projectile, wind = wind,
                shot = shot, target = destination, max_steps = config.max_steps,
            })
            local overlaps_muzzle = (shot.x - destination.x) ^ 2 + (shot.y - destination.y) ^ 2 <= destination.radius ^ 2
            if overlaps_muzzle then
                result = trajectory.simulate({
                    map_id = fight_state.map_id, projectile_id = projectile, wind = wind,
                    shot = shot, target = {x = destination.x, y = destination.y, radius = 0.01},
                    max_steps = config.max_steps,
                })
            end
            if result.status ~= "target" then
                result.distance = math.sqrt((result.x - destination.x) ^ 2 + (result.y - destination.y) ^ 2)
            end
            if result.status == "bounds" or result.status == "limit" or result.time == 0 then
                result.distance = math.huge
            end
            return result
        end

        local function height(shot)
            return aim_compute(job.heights, shot_key(shot), function()
                return height_at_target(model, shot, destination, direction)
            end)
        end

        local function predict(shot)
            local result = aim_compute(job.predictions, shot_key(shot), function()
                return predict_uncached(shot)
            end)
            if allow_risk then
                local candidate = {
                    shot = shot, direction = direction, trident = use_trident == true,
                    risky = true, distance = result.distance,
                    direct = result.status == "target" and result.time > 0,
                }
                if better_shot(candidate, closest) then closest = candidate end
            end
            return result
        end

        local function clear(physics, shot, variant)
            return aim_compute(job.clearance, variant .. ":" .. shot_key(shot), function()
                return clears_player(physics, shot, self, nearby, minimum_clearance)
            end)
        end

        for _, preferred in ipairs(rotated((offset and offset.angles) or config.angles, stuck_shift(fight_state))) do
            local angle = math.max(math.ceil(weapon.min_angle), math.min(math.floor(weapon.max_angle), preferred))
            if not tried[angle] then
                tried[angle] = true
                local function launch(force)
                    return trajectory.launch({
                        x = self.position.x, y = self.position.y, direction = direction,
                        slope = slope, gun_angle = angle, force = force,
                    })
                end

                local low, high = 1, 2000
                if height(launch(high)) <= 0 then
                    while high - low > 1 do
                        local force = math.floor((low + high) / 2)
                        if height(launch(force)) > 0 then
                            low = force
                        else
                            high = force
                        end
                    end

                    for force = math.max(1, low - 2), math.min(2000, high + 2) do
                        local shot = launch(force)
                        local result = predict(shot)
                        local safe = result.status == "target" and result.time > 0
                        if safe and not allow_risk then safe = clear(model, shot, "normal") end
                        if safe and use_trident and not allow_risk then
                            local multi = trajectory.model({map_id = fight_state.map_id, projectile_id = weapon.multi, wind = wind})
                            safe = clear(multi, {x = shot.x, y = shot.y, force = trunc(shot.force * 0.9), angle = shot.angle - 5}, "left")
                                and clear(multi, {x = shot.x, y = shot.y, force = trunc(shot.force * 1.1), angle = shot.angle + 5}, "right")
                        end
                        if safe then
                            local solution = {shot = shot, direction = direction, trident = use_trident == true, risky = allow_risk == true, distance = result.distance, direct = true}
                            state.aim_cache[cache_key] = solution
                            return solution
                        end
                    end
                end
                if allow_risk and (not closest or not closest.direct) then
                    for force = 1, 2000, 50 do
                        local result = predict(launch(force))
                        if result.status == "target" and result.time > 0 then break end
                    end
                end
            end
        end

        if closest then
            state.aim_cache[cache_key] = closest
            return closest
        end
        job.failed = true
        error({code = "formigueiro_no_shot", message = "No clear shot found for target " .. target.id}, 0)
    end

    local function target_candidates(self, fight_state, ignore_only)
        local candidates = {}
        for _, actor in ipairs(fight_state.actors) do
            local hp = actor.pending_hp
            if hp == nil then hp = actor.hp end
            local position = actor.position
            local source = "position"
            if not actor.player_id and actor.destination then
                position = actor.destination
                source = "server_destination"
            end
            if actor.team and actor.team ~= self.team and position
                and actor.alive ~= false and hp and hp > 0 then
                candidates[#candidates + 1] = {
                    id = actor.id, max_hp = actor.max_hp, position = {x = position.x, y = position.y},
                    position_source = source, name = actor.name,
                    first = name_rank(actor.name, config.target_first) or 1000,
                    last = name_rank(actor.name, config.target_last) and 1 or 0,
                }
            end
        end

        local only = not ignore_only and config.target_only[tostring(fight_state.map_id)]
        if only then
            local chosen = {}
            for _, candidate in ipairs(candidates) do
                if name_rank(candidate.name, {only}) then chosen[#chosen + 1] = candidate end
            end
            if #chosen > 0 then candidates = chosen end
        end

        -- Estágio 1: os pintinhos se dividem em grupos à esquerda e à direita; mira no maior grupo (mais vizinhos
        -- a até "radius" px), para o tridente acertar vários de uma vez (pedido do usuário, 05/11).
        local cluster = config.cluster_maps[tostring(fight_state.map_id)]
        for _, a in ipairs(candidates) do
            a.neighbors = 0
            if cluster then
                for _, b in ipairs(candidates) do
                    if a ~= b and math.abs(a.position.x - b.position.x) <= cluster.radius
                        and math.abs(a.position.y - b.position.y) <= cluster.radius then
                        a.neighbors = a.neighbors + 1
                    end
                end
            end
        end

        table.sort(candidates, function(a, b)
            if a.last ~= b.last then return a.last < b.last end
            if a.first ~= b.first then return a.first < b.first end
            if a.neighbors ~= b.neighbors then return a.neighbors > b.neighbors end

            local a_distance = math.abs(a.position.x - self.position.x)
            local b_distance = math.abs(b.position.x - self.position.x)
            if a_distance == b_distance then return a.id < b.id end
            return a_distance < b_distance
        end)

        return candidates
    end

    local function plan_props(self, fight_state)
        local plan = {}
        local energy = self.energy
        if not energy then
            state.damage_reason = "Stamina is unknown"
            return plan
        end
        state.prop_origin = {x = self.position.x, y = self.position.y}
        local primary = extra_attack
        if uses_trident(fight_state.map_id) then primary = trident end
        if stage_plan(fight_state.map_id) and not uses_trident(fight_state.map_id) then primary = nil end
        local cost = primary and tonumber(primary.properties[4]) or math.huge
        if primary and energy >= cost and (primary.id ~= trident.id or (state.trident_allowed and not self.special)) then
            plan[#plan + 1] = primary
            energy = energy - cost
        end
        local extras = stage_plan(fight_state.map_id)
        if extras then
            local known = {extra_attack_2 = extra_attack_2, extra_attack = extra_attack, max_damage = max_damage}
            local turn = state.my_turns[tostring(fight_state.map_id)] or 0
            for _, entry in ipairs(extras) do
                local name, from = entry, nil
                if type(entry) == "table" then name, from = entry.name, entry.from_turn end
                local prop = known[name]
                cost = tonumber(prop.properties[4]) or 0
                if (not from or turn >= from) and energy >= cost then
                    plan[#plan + 1] = prop
                    energy = energy - cost
                end
            end
            if extras.only_until_turn and turn <= extras.only_until_turn then return plan end
        end
        if config.no_damage_maps[tostring(fight_state.map_id)] then return plan end
        for _, prop in ipairs(damage_props) do
            cost = tonumber(prop.properties[4])
            while energy >= cost do
                plan[#plan + 1] = prop
                energy = energy - cost
                if uses_trident(fight_state.map_id) and not extras then return plan end
            end
        end
        return plan
    end

    local function prepare_buffs(self, target, fight_state)
        local controls = self.controls or {}
        if state.buff_waiting then
            state.reason = "Waiting for server confirmation of " .. state.buff_waiting.kind
            return false
        end

        if not state.prop_plan then
            state.prop_plan = plan_props(self, fight_state)
            state.prop_index = 1
        end

        if not state.pow then
            state.pow = "not_charged"
            if self.special then
                state.pow = "confirmed"
            elseif state.prop_plan[1] and state.prop_plan[1].id == trident.id
                and controls.known and controls.right_prop_enabled and not controls.lock_right_prop
                and not controls.lock_state and not controls.frozen then
                state.pow = "trident_selected"
            elseif uses_trident(fight_state.map_id) and not state.trident_allowed then
                state.pow = "single_shot_selected"
            elseif not controls.known then
                state.pow = "controls_unknown"
            elseif controls.lock_spell_kill or controls.frozen then
                state.pow = "locked"
            elseif self.dander and self.dander >= 200 then
                local weapon = catalog.weapon({id = self.weapon_id})
                local supported, problem = pcall(aim, self, target, fight_state, weapon.special)
                if supported then
                    send(fight.pow)
                    state.buff_action = state.last_action
                    state.pow = "requested"
                    state.buff_waiting = {kind = "POW", actor_id = self.id, started = now()}
                    state.buff_message = nil
                    state.reason = "Waiting for the POW projectile"
                    return false
                end
                if type(problem) ~= "table" or (problem.code ~= "trajectory_unsupported"
                    and problem.code ~= "catalog_unavailable" and problem.code ~= "formigueiro_no_shot") then
                    error(problem, 0)
                end
                state.pow = problem.code
            end
        end

        if state.props_done then return true end
        if not controls.known or not controls.right_prop_enabled or controls.lock_right_prop
            or controls.lock_state or controls.frozen then
            state.props_done = true
            state.damage_reason = "Prop controls are unknown or locked"
            return true
        end
        if state.prop_origin and (self.position.x ~= state.prop_origin.x or self.position.y ~= state.prop_origin.y) then
            state.props_done = true
            state.damage_reason = "Player moved; remaining stamina is unconfirmed"
            return true
        end

        local prop = state.prop_plan[state.prop_index]
        if prop then
            state.prop_index = state.prop_index + 1
            local arguments = {bag_type = 1, place = -1, template_id = prop.id}
            send(fight.use_prop, arguments)
            if prop.id == trident.id then state.trident_requested = true end
            state.buff_action = state.last_action
            state.buff_waiting = {
                kind = "damage buff", actor_id = self.id, arguments = arguments, started = now(),
            }
            state.buff_message = nil
            state.damage_reason = "Waiting for server confirmation"
            state.reason = state.damage_reason
            return false
        end
        state.props_done = true

        return true
    end

    local function check_buff_wait(packet)
        local waiting = state.buff_waiting
        if not waiting then return end
        if packet and packet.direction == "ServerToClient" and packet.status == "decoded" and packet.code == 3 then
            state.buff_message = packet.fields.message
        end

        local current = fight.state()
        local key = current.generation .. ":" .. current.turn_revision
        if key ~= state.turn_key or not current.turn or current.turn.actor_id ~= waiting.actor_id then
            skip_buff("Turn ended before confirming " .. waiting.kind)
            return
        end

        if (now() - waiting.started) >= config.buff_timeout then
            skip_buff("Unconfirmed " .. waiting.kind .. "; continuing without waiting or retrying")
        end
    end

    local function observe_buffs(packet)
        if packet.status ~= "decoded" then return end
        local fields = packet.fields
        local waiting = state.buff_waiting
        if not waiting or packet.code ~= 91 then return end
        local current = fight.state()
        local key = current.generation .. ":" .. current.turn_revision
        if key ~= state.turn_key or not current.turn or current.turn.actor_id ~= waiting.actor_id then return end
        if fields.living_id ~= waiting.actor_id then return end

        if waiting.kind == "POW" and fields.subcommand == 20 then
            if fields.special ~= true then
                skip_buff("The server did not activate POW")
                return
            end
            state.pow = "confirmed"
            state.buff_waiting = nil
        elseif waiting.kind == "damage buff" and fields.subcommand == 32 then
            local expected = waiting.arguments
            local same_slot = fields.bag_type == expected.bag_type and fields.place == expected.place
            local free_prop = expected.bag_type == 1 and expected.place == -1
                and fields.bag_type == -2 and fields.place == -2
            if (same_slot or free_prop) and fields.template_id == expected.template_id then
                state.damage_buff = fields.template_id
                state.confirmed_props[#state.confirmed_props + 1] = fields.template_id
                state.damage_reason = "confirmed"
                state.buff_waiting = nil
            end
        end
    end

    local function can_plan_trident(self, fight_state)
        if not uses_trident(fight_state.map_id) or self.special then return false end
        if state.props_done or state.prop_plan then return false end

        local controls = self.controls or {}
        if not controls.known or not controls.right_prop_enabled then return false end
        if controls.lock_right_prop or controls.lock_state or controls.frozen then return false end

        return (self.energy or 0) >= tonumber(trident.properties[4])
    end

    -- ===== Balões (estágio 1) =====
    -- Pacotes 91 que o relay da Olimpíadas entrega com os bytes crus (raw_hex): 48 = objeto novo (id, tipo, x, y,
    -- modelo, ação), 223 = ação do objeto ("shield<valor>": valor do balão, -5..5), 53 = objeto removido.
    local function hex_i32(h, o)
        local v = tonumber(string.sub(h, o, o + 7), 16)
        if not v then return nil end
        if v >= 2147483648 then v = v - 4294967296 end
        return v
    end

    local function hex_utf(h, o)
        local n = tonumber(string.sub(h, o, o + 3), 16)
        if not n then return "", o end
        local out = {}
        for i = 0, n - 1 do
            local byte = tonumber(string.sub(h, o + 4 + 2 * i, o + 5 + 2 * i), 16)
            if byte and byte > 0 then out[#out + 1] = string.char(byte) end
        end
        return table.concat(out), o + 4 + 2 * n
    end

    local function observe_balloons(packet)
        if packet.code ~= 91 or packet.direction ~= "ServerToClient" or type(packet.fields) ~= "table" then return end
        local sub = packet.fields.subcommand
        -- placar do estágio (BARRIER_INFO): rodada e pontos
        if sub == 104 and type(packet.fields.current_value_2) == "number" then
            state.balloon_points = packet.fields.current_value_2
            state.balloon_round = packet.fields.current_value_1
            return
        end
        local h = packet.fields.raw_hex
        if type(h) ~= "string" then return end
        if sub == 48 then
            local id, x, y = hex_i32(h, 1), hex_i32(h, 17), hex_i32(h, 25)
            local model = hex_utf(h, 33)
            if id and x and y and string.find(model, "six.ball", 1, true) then
                state.balloons[id] = {x = x, y = y}
                state.balloon_picked[id] = nil
            end
        elseif sub == 223 then
            local id = hex_i32(h, 1)
            local _, o = hex_utf(h, 9)
            local action = hex_utf(h, o)
            local value = tonumber(string.match(action, "shield(%-?%d+)"))
            if id and value then state.balloon_values[id] = value end
        elseif sub == 53 then
            local id = hex_i32(h, 1)
            if id then
                state.balloons[id] = nil
                state.balloon_values[id] = nil
                state.balloon_picked[id] = nil
            end
        end
    end

    local function balloon_list()
        local list = {}
        for id, b in pairs(state.balloons) do
            local value = state.balloon_values[id]
            if value then list[#list + 1] = {id = id, x = b.x, y = b.y, value = value} end
        end
        table.sort(list, function(a, b) return a.id < b.id end)
        return list
    end

    -- Trajetória sem terreno (o céu do 1165 é aberto): índices dos balões a até "radius" px do caminho.
    local function balloon_path(model, shot, list, radius, ground_y)
        local rad = math.rad(shot.angle)
        local x, y = shot.x, shot.y
        local vx, vy = trunc(shot.force * math.cos(rad)), trunc(shot.force * math.sin(rad))
        local r2 = radius * radius
        local hit = {}
        local low, high = math.huge, -math.huge
        for _, b in ipairs(list) do
            low = math.min(low, b.y - radius)
            high = math.max(high, b.y + radius)
        end
        for _ = 1, 500 do
            vx = vx + (model.wind_force - model.drag * vx) / model.mass * model.dt
            vy = vy + (model.gravity_force - model.drag * vy) / model.mass * model.dt
            x, y = x + vx * model.dt, y + vy * model.dt
            if y >= low and y <= high then
                for i, b in ipairs(list) do
                    if not hit[i] and (x - b.x) ^ 2 + (y - b.y) ^ 2 <= r2 then hit[i] = true end
                end
            end
            if y > ground_y or x < 0 or x > model.width then break end
        end
        return hit
    end

    local function balloon_score(sets, list, picked, rules)
        local seen, score, ids = {}, 0, {}
        for _, set in ipairs(sets) do
            for i in pairs(set) do
                local b = list[i]
                if not seen[i] and not picked[b.id] then
                    seen[i] = true
                    ids[#ids + 1] = b.id
                    local v = b.value
                    if v == 0 then v = rules.zero_value elseif v < 0 then v = v * rules.negative_weight end
                    score = score + v
                end
            end
        end
        return score, ids
    end

    -- Melhor tiro nos balões: grade de ângulos (passo 2) e forças (passo 20), refinada perto dos 12 melhores;
    -- com tridente soma os 3 caminhos (laterais: força x0,9/x1,1 e ângulo -5/+5, projétil "multi").
    local function balloon_shot(self, fight_state, use_trident)
        local rules = config.balloon_maps[tostring(fight_state.map_id)]
        local list = balloon_list()
        if #list == 0 then return nil end
        local weapon = catalog.weapon({id = self.weapon_id})
        local projectile = self.ball_id or weapon.common
        local observed_wind = fight.wind()
        local wind = config.fallback_wind
        if observed_wind.current and type(observed_wind.value) == "number" then wind = observed_wind.value end
        state.wind_used = wind
        local model = trajectory.model({map_id = fight_state.map_id, projectile_id = projectile, wind = wind})
        local multi = use_trident and trajectory.model({map_id = fight_state.map_id, projectile_id = weapon.multi, wind = wind})
        local slope = trajectory.slope({map_id = fight_state.map_id, x = self.position.x, y = self.position.y})
        local mean = 0
        for _, b in ipairs(list) do mean = mean + b.x end
        local direction = 1
        if (mean / #list) < self.position.x then direction = -1 end
        local cache = {}
        local function sets_for(angle, force)
            local key = angle .. ":" .. force
            if cache[key] then return cache[key] end
            local shot = trajectory.launch({x = self.position.x, y = self.position.y, direction = direction,
                slope = slope, gun_angle = angle, force = force})
            local sets = {balloon_path(model, shot, list, rules.radius, rules.ground_y)}
            if multi then
                sets[2] = balloon_path(multi, {x = shot.x, y = shot.y, force = trunc(shot.force * 0.9), angle = shot.angle - 5}, list, rules.radius, rules.ground_y)
                sets[3] = balloon_path(multi, {x = shot.x, y = shot.y, force = trunc(shot.force * 1.1), angle = shot.angle + 5}, list, rules.radius, rules.ground_y)
            end
            cache[key] = {shot = shot, sets = sets}
            return cache[key]
        end
        local picked = state.balloon_picked
        local min_angle, max_angle = math.ceil(weapon.min_angle), math.floor(weapon.max_angle)
        local top = {}
        for angle = min_angle, max_angle, 2 do
            for force = 300, 2000, 20 do
                local score = balloon_score(sets_for(angle, force).sets, list, picked, rules)
                top[#top + 1] = {score = score, angle = angle, force = force}
            end
        end
        table.sort(top, function(a, b) return a.score > b.score end)
        local best
        for i = 1, math.min(12, #top) do
            local t = top[i]
            for angle = math.max(min_angle, t.angle - 1), math.min(max_angle, t.angle + 1) do
                for force = math.max(1, t.force - 15), math.min(2000, t.force + 15), 5 do
                    local entry = sets_for(angle, force)
                    local score, ids = balloon_score(entry.sets, list, picked, rules)
                    -- desempate: no meio da faixa boa (vizinhos de força com a mesma nota), contra o erro da física
                    local margin = 0
                    for _, d in ipairs({-10, 10}) do
                        local f = force + d
                        if f >= 1 and f <= 2000 and balloon_score(sets_for(angle, f).sets, list, picked, rules) >= score then
                            margin = margin + 1
                        end
                    end
                    if not best or score > best.score or (score == best.score and margin > best.margin) then
                        best = {score = score, margin = margin, ids = ids, shot = entry.shot, angle = angle, direction = direction}
                    end
                end
            end
        end
        return best
    end

    -- 12/11 (usuário: "no Normal o chefe não dá tanto dano; ela está batendo em si mesma"): anota cada queda de
    -- vida dela com o turno e de quem era a vez (vez dela = dano do próprio tiro).
    local function observe_hp()
        local me = fight.self()
        local current = fight.state()
        if not me or type(me.hp) ~= "number" then return end
        if state.hp_map ~= current.map_id then
            state.hp_map = current.map_id
            state.hp_seen = me.hp
            return
        end
        if state.hp_seen and me.hp < state.hp_seen then
            local turn = current.turn or {}
            table.insert(state.hp_log, 1, string.format("%s turno=%s vez=%s%s -%d (%d)", tostring(current.map_id),
                tostring(turn.number), tostring(turn.actor_id), turn.actor_id == me.id and "(ela)" or "",
                state.hp_seen - me.hp, me.hp))
            state.hp_log[31] = nil
        end
        state.hp_seen = me.hp
    end

    local function attack(fight_state)
        state.search_deadline = now() + 0.09  -- 90 ms de cada tick de 100 ms
        local self = fight.self()
        if not self then
            state.reason = "Waiting for the local fight actor"
            return
        end
        if not fight_state.turn or fight_state.turn.actor_id ~= self.id then
            state.reason = "Waiting for the player turn"
            return
        end
        if self.alive == false or (self.hp and self.hp <= 0) then
            state.reason = "Waiting for the mission result after player death"
            return
        end

        local key = fight_state.generation .. ":" .. fight_state.turn_revision
        if config.attack_policy == "leader" and not party:is_leader()
            and not config.all_attack_maps[tostring(fight_state.map_id)] then
            -- 11/11 (aprovado pelo usuário): derrotas por tempo no estágio 1 com um inimigo grudado na principal,
            -- que fica sem tiro limpo, enquanto convidadas vivas só passavam a vez. Num estágio com "stuck_turns",
            -- a partir desse número de turnos da convidada no estágio ela atira também, com a mira normal.
            local map = tostring(fight_state.map_id)
            if state.follower_turn_key ~= key then
                state.follower_turn_key = key
                state.follower_turns = state.follower_turns or {}
                state.follower_turns[map] = (state.follower_turns[map] or 0) + 1
            end
            local limit = config.stuck_turns[map]
            if not (limit and state.follower_turns[map] >= limit) then
                if state.passed_turn ~= key then
                    send(fight.pass, {argument = 0})
                    state.passed_turn = key
                end
                state.attack_mode = "party_pass"
                state.reason = "Waiting for the server to end the follower turn"
                return
            end
            state.follower_note = string.format("stage %s: follower attacking from turn %d", map, state.follower_turns[map])
        end

        if state.turn_key ~= key then
            if state.turn_key ~= "" and state.fired_turn == state.turn_key then
                cronometrar(string.format("fim do turno: %d tiro(s) aceitos de %d", state.server_shots or 0,
                    1 + (state.followups or 0)))
            end
            state.turn_key = key
            if state.fly_prop_sent and state.fly_prop_sent ~= key then state.fly_prop_sent = nil; state.fly_retries = 0 end
            state.my_turns[tostring(fight_state.map_id)] = (state.my_turns[tostring(fight_state.map_id)] or 0) + 1
            state.turn_started = now()
            state.aim_cache = {}
            state.aim_jobs = {}
            state.pow = nil
            state.prop_plan = nil
            state.prop_origin = nil
            state.prop_index = 1
            state.buff_rechecks = 0
            state.props_done = false
            state.confirmed_props = {}
            state.buff_issue = nil
            state.damage_buff = nil
            state.damage_reason = nil
            state.buff_waiting = nil
            state.followup_started = nil
            state.followup_sent = false
            state.server_shots = 0
            state.followups = 0
            state.max_shots = 1
            state.shot_followup = false
            state.trident_requested = false
            state.trident_allowed = false
        end
        state.max_shots = math.max(state.max_shots or 1, self.shoot_count or 1)
        if state.fired_turn == key then
            local extra = state.max_shots - 1
            if state.followup_started and not state.followup_sent and extra >= 1
                and (state.followups or 0) < extra and (state.server_shots or 0) > (state.followups or 0)
                and (now() - state.followup_started) >= config.followup_delay then
                if config.balloon_maps[tostring(fight_state.map_id)] then
                    -- balões: cada tiro extra recalcula a mira sem os balões já pegos (pedido do usuário)
                    local again = balloon_shot(self, fight_state, state.balloon_trident)
                    if again and again.score > 0 then
                        for _, id in ipairs(again.ids) do state.balloon_picked[id] = true end
                        state.shot = again.shot
                        state.balloon_note = (state.balloon_note or "") .. string.format(" | extra %.1f [%s]", again.score, table.concat(again.ids, ","))
                    end
                end
                send(fight.shoot, state.shot)
                cronometrar(string.format("%s extra %.2fs (espera %.2fs)", tostring(fight_state.map_id),
                    now() - (state.shot_sent_at or now()), now() - state.followup_started))
                state.shot_sent_at = now()
                state.shot_action = state.last_action
                state.shot_followup = true
                state.followups = (state.followups or 0) + 1
                state.followup_started = nil
                state.followup_sent = state.followups >= extra
                state.reason = "Waiting for the additional shot result"
            end
            return
        end

        state.player_position_source = "server_position"
        -- O servidor aceita o andar, mas não devolve a posição a quem andou (só no turno seguinte): neste turno,
        -- mira a partir do destino pedido e estima a energia gasta (~2 por px; vídeo do usuário: 80 → ~35 px).
        if self.destination and state.walk_sent and state.walk_sent.key == key then
            local real = self
            local dest = {x = state.walk_sent.x, y = state.walk_sent.y}
            local spent = 2 * math.abs(state.walk_sent.x - state.walk_sent.from)
            local energy = state.walk_sent.energy and math.max(0, state.walk_sent.energy - spent) or nil
            self = setmetatable({position = dest, destination = false, energy = energy},
                {__index = function(_, k) return real[k] end})
            state.player_position_source = "walk_destination"
        end
        if not self.position or self.destination then
            state.reason = "Waiting for server-confirmed player position"
            return
        end
        if state.buff_waiting then return end
        if not self.weapon_id then
            state.reason = "Waiting for the equipped weapon"
            return
        end
        if not self.team then
            state.reason = "Waiting for the player team"
            return
        end

        if config.balloon_maps[tostring(fight_state.map_id)] then
            -- Balões (pedido do usuário, 12/11): tridente + 2 em todas as contas; o POW não serve aqui.
            state.pow = state.pow or "balloons"
            state.trident_allowed = true
            local list = balloon_list()
            if #list == 0 then
                state.reason = "Waiting for the balloons of this round"
                return
            end
            -- 13/11: a mira leva ~0,5 s; calculada depois dos itens, o servidor recusava o tiro (estado mudou) e o motor
            -- refazia a conta até 4 vezes (tiro com 3 s). Agora calcula uma vez no começo do turno (com tridente, que é o
            -- plano) e reaproveita no envio e nas repetições.
            if #list < 12 and now() - (state.turn_started or now()) < 1.5 then
                state.reason = "Waiting for all 12 balloons of this round"
                return
            end
            if state.balloon_best_key ~= key then
                state.balloon_picked = {}
                state.balloon_best = balloon_shot(self, fight_state, true)
                state.balloon_best_key = key
            end
            if not prepare_buffs(self, {id = -1, position = {x = list[1].x, y = list[1].y}}, fight_state) then return end
            local with_trident = false
            for _, id in ipairs(state.confirmed_props or {}) do
                if id == trident.id then with_trident = true end
            end
            state.balloon_trident = with_trident
            -- 1º tiro do turno: nada foi pego ainda (um tiro recusado pelo servidor volta aqui e não conta).
            state.balloon_picked = {}
            local best = state.balloon_best
            if not with_trident then best = balloon_shot(self, fight_state, false) end
            if not best then
                state.reason = "Waiting for the balloons of this round"
                return
            end
            for _, id in ipairs(best.ids) do state.balloon_picked[id] = true end
            local positives = 0
            for _, b in ipairs(list) do if b.value > 0 then positives = positives + b.value end end
            state.balloon_note = string.format("turn %d: score %.1f of %d (%d balloons) trident=%s [%s] angle=%d force=%d wind=%s me@%d,%d points=%s",
                state.my_turns[tostring(fight_state.map_id)] or 0, best.score, positives, #list, tostring(with_trident),
                table.concat(best.ids, ","), best.angle, best.shot.force, tostring(state.wind_used), self.position.x, self.position.y, tostring(state.balloon_points))
            table.insert(state.balloon_log, 1, state.balloon_note)
            state.balloon_log[21] = nil
            local elapsed = math.min(127, math.floor((now() - state.turn_started)))
            send(fight.face, {direction = best.direction})
            send(fight.shoot_tag, {started = true, shot_type = elapsed})
            send(fight.shoot, best.shot)
            state.shot_action = state.last_action
            state.shot_followup = false
            state.shot_sent_at = now()
            cronometrar(string.format("%s balões %.2fs nota=%.1f itens=%d", tostring(fight_state.map_id),
                now() - state.turn_started, best.score, #(state.confirmed_props or {})))
            state.attack_mode = "balloons"
            state.fired_turn = key
            state.target_id = -1
            state.shot = best.shot
            state.reason = "Waiting for the shot result and next turn"
            return
        end

        local fly = config.fly_maps[tostring(fight_state.map_id)]
        local fly_key = tostring(fight_state.map_id)
        if fly and not state.flown[fly_key] then
            local controls = self.controls or {}
            local house
            for _, actor in ipairs(fight_state.actors) do
                local hp = actor.pending_hp
                if hp == nil then hp = actor.hp end
                if name_rank(actor.name, {fly.target}) and actor.alive ~= false and hp and hp > 0 then
                    house = actor.destination or actor.position
                end
            end
            local energy = self.energy or self.max_energy or 300  -- no 1º turno a energia ainda não veio
            if house and fly.only_within and math.abs(house.x - self.position.x) > fly.only_within then
                state.flown[fly_key] = "not_needed"
                state.fly_note = string.format("no fly: %d px from %s", math.abs(house.x - self.position.x), fly.target)
            elseif not house or energy < 150 or not controls.known or not controls.right_prop_enabled
                or controls.lock_right_prop or controls.lock_state or controls.frozen then
                state.flown[fly_key] = "skipped"
                state.fly_note = string.format("skipped: house=%s energy=%s controls=%s/%s/%s/%s/%s",
                    tostring(house ~= nil), tostring(self.energy), tostring(controls.known), tostring(controls.right_prop_enabled),
                    tostring(controls.lock_right_prop), tostring(controls.lock_state), tostring(controls.frozen))
            else
                local side = 1
                if self.position.x < house.x then side = -1 end
                local land_x = house.x + side * fly.standoff
                local model_map = fight_state.map_id
                local hit = terrain.trace({map_id = model_map, x = land_x, y = 0, to_x = land_x, to_y = 2000})
                if not hit.hit then
                    state.flown[fly_key] = "no_ground"
                    state.fly_note = "no ground at x=" .. land_x
                elseif not state.fly_prop_sent then
                    send(fight.use_prop, {bag_type = 1, place = -1, template_id = config.fly_prop})
                    state.fly_prop_sent = key
                    state.fly_action = state.last_action  -- recusado: tenta de novo (ver o tratamento dos resultados)
                    state.reason = "Flying: fly prop requested"
                    return
                elseif state.fly_prop_sent == key then
                    local spot = {id = -1, position = {x = land_x, y = math.floor(hit.y)}, position_source = "fly_landing"}
                    local found, solution = pcall(aim, self, spot, fight_state, config.fly_projectile, false, true)
                    state.flown[fly_key] = true
                    if found and solution then
                        local elapsed = math.min(127, math.floor((now() - state.turn_started)))
                        send(fight.face, {direction = solution.direction})
                        send(fight.shoot_tag, {started = true, shot_type = elapsed})
                        send(fight.shoot, solution.shot)
                        state.shot_action = state.last_action
                        state.shot_followup = false
                        state.fired_turn = key
                        state.followup_sent = true
                        state.target_id = -1
                        state.shot = solution.shot
                        state.attack_mode = "fly"
                        state.fly_note = string.format("fly shot to x=%d y=%d force=%s angle=%s", land_x, math.floor(hit.y), tostring(solution.shot.force), tostring(solution.shot.angle))
                        state.reason = "Flying next to " .. fly.target
                        return
                    end
                    if type(solution) == "table" and solution.code == "formigueiro_aim_pending" then
                        state.flown[fly_key] = nil
                        error(solution, 0)
                    end
                    state.fly_note = "no fly trajectory: " .. tostring(type(solution) == "table" and solution.message or solution)
                end
            end
        end

        local walk = config.walk_back[tostring(fight_state.map_id)]
        local walk_key = tostring(fight_state.map_id)
        -- 08/11 (pedido do usuário): não chegou ao ponto no turno do recuo (só anda ~40 px por turno)? Anda de
        -- novo nos turnos seguintes, até o turno "max_turn" dela, antes dos itens e do tiro.
        if walk and walk.spot and state.walked[walk_key] == true and state.walk_goal and state.walk_done_key ~= key
            and (state.my_turns[walk_key] or 0) <= (walk.max_turn or walk.turn) and self.position
            and math.abs(state.walk_goal - self.position.x) >= 10 then
            state.walked[walk_key] = nil
            state.walk_sent = nil
            state.walk_skip_turn = nil
        end
        if walk and state.flown[walk_key] == true and not state.walked[walk_key]
            and (state.my_turns[walk_key] or 0) >= walk.turn
            and (state.my_turns[walk_key] or 0) <= (walk.max_turn or math.huge) then
            if state.walk_sent and state.walk_sent.key ~= key then
                -- O turno do recuo acabou sem confirmação: no turno novo o servidor já devolveu a posição real.
                if self.position and math.abs(self.position.x - state.walk_sent.from) >= 10 then
                    state.walked[walk_key] = true
                    state.walk_done_key = key
                    state.walk_note = (state.walk_note or "") .. " -> confirmed next turn at x=" .. self.position.x
                else
                    state.walk_note = (state.walk_note or "") .. " -> retry next turn"
                    state.walk_sent = nil  -- tenta de novo neste turno (até dar certo)
                    state.walk_retries = 0
                end
            end
            if state.walked[walk_key] then
                -- já andou (confirmado acima): segue para o tiro
            elseif not state.walk_sent then
                local house
                for _, actor in ipairs(fight_state.actors) do
                    if name_rank(actor.name, {config.fly_maps[walk_key] and config.fly_maps[walk_key].target or ""}) then
                        house = actor.destination or actor.position
                    end
                end
                local side = 1
                if house and self.position.x < house.x then side = -1 end
                -- Anda o máximo que a energia deixa (2 por px) guardando "keep_energy" para os itens do turno,
                -- até "distance" px; energia desconhecida ou curta: "min_distance" (o recuo de antes).
                local distance = walk.min_distance or walk.distance
                if type(self.energy) == "number" and walk.keep_energy then
                    distance = math.max(distance, math.min(walk.distance, math.floor((self.energy - walk.keep_energy) / 2)))
                end
                -- 08/11: com "spot", anda em direção ao ponto a "spot" px da casa (nunca passa dele).
                local goal
                if house and walk.spot then
                    goal = house.x + side * walk.spot
                    if goal < self.position.x then side = -1 else side = 1 end
                    distance = math.min(distance, math.abs(goal - self.position.x))
                    state.walk_goal = goal
                end
                local nx = self.position.x + side * distance
                local hit = terrain.trace({map_id = fight_state.map_id, x = nx, y = math.max(0, self.position.y - 120),
                    to_x = nx, to_y = self.position.y + 300})
                if goal and distance < 10 then
                    state.walked[walk_key] = true
                    state.walk_note = string.format("already at x=%d (spot x=%d): no walk", self.position.x, goal)
                elseif not house or not hit.hit or not fight_state.turn or not fight_state.turn.number then
                    state.walked[walk_key] = "skipped"
                    state.walk_note = "walk skipped: house/ground/turn unknown"
                else
                    local ny = math.floor(hit.y)
                    send(fight.move_start, {moving = true, move_type = 0, x = nx, y = ny, direction = side,
                        is_living = true, turn_number = fight_state.turn.number})
                    state.walk_action = state.last_action  -- recusado: tenta de novo (ver o tratamento dos resultados)
                    state.walk_sent = {key = key, x = nx, y = ny, from = self.position.x, at = now(), energy = self.energy}
                    state.walk_note = string.format("walking from x=%d to x=%d y=%d", self.position.x, nx, ny)
                    state.reason = "Walking back before shooting"
                    return
                end
            elseif state.walk_skip_turn ~= key then
                local moved = math.abs(self.position.x - state.walk_sent.from) >= 10
                if moved then
                    state.walked[walk_key] = true
                    state.walk_done_key = key
                    state.walk_note = state.walk_note .. " -> now x=" .. self.position.x
                        .. string.format(" (energy %s -> %s)", tostring(state.walk_sent.energy), tostring(self.energy))
                elseif now() - state.walk_sent.at >= 2.5 then
                    -- Não andou neste turno: atira sem tridente (uses_trident) e tenta andar de novo no próximo.
                    state.walk_skip_turn = key
                    state.walk_note = state.walk_note .. " -> server did not move; retry next turn"
                else
                    state.reason = "Waiting for the walk to finish"
                    return
                end
            end
        end

        -- Chefes (regra do usuário, 12/11): colada no chefe ela se explode junto. "É só afastar um pouco e bater na
        -- cabeça (em cima do boss) ou afastar para as costas." Perto (menos de "radius" px): anda para longe até
        -- "max_walk" px guardando "keep_energy" para os itens e mira na cabeça (tiro alto descendo por cima).
        local step = config.close_step[tostring(fight_state.map_id)]
        state.close_head = false
        -- 13/11: com a Mesa dos Juízes viva, não anda (o passo gastou o turno da Mesa numa derrota): atira nela.
        if step and step.skip_when then
            for _, actor in ipairs(fight_state.actors) do
                local hp = actor.pending_hp
                if hp == nil then hp = actor.hp end
                if name_rank(actor.name, {step.skip_when}) and actor.alive ~= false and hp and hp > 0 then step = nil; break end
            end
        end
        if step then
            local boss
            for _, actor in ipairs(fight_state.actors) do
                local hp = actor.pending_hp
                if hp == nil then hp = actor.hp end
                if name_rank(actor.name, {step.target}) and actor.alive ~= false and hp and hp > 0 then
                    boss = actor.destination or actor.position
                end
            end
            if boss and math.abs(boss.x - self.position.x) < step.radius then
                state.close_head = true
                if state.step_key ~= key then
                    local side = 1
                    if boss.x > self.position.x then side = -1 end
                    local distance = step.max_walk
                    if type(self.energy) == "number" then
                        distance = math.min(step.max_walk, math.floor((self.energy - step.keep_energy) / 2))
                    end
                    local nx = self.position.x + side * distance
                    local hit = terrain.trace({map_id = fight_state.map_id, x = nx, y = math.max(0, self.position.y - 120),
                        to_x = nx, to_y = self.position.y + 300})
                    state.step_key = key
                    state.step_sent = nil
                    if distance >= 10 and hit.hit and fight_state.turn and fight_state.turn.number then
                        local ny = math.floor(hit.y)
                        send(fight.move_start, {moving = true, move_type = 0, x = nx, y = ny, direction = side,
                            is_living = true, turn_number = fight_state.turn.number})
                        state.walk_action = state.last_action  -- 13/11: recusado (stale_state) pausava o bot; agora repete
                        state.step_sent = {x = nx, y = ny, from = self.position.x, at = now(),
                            energy = self.energy and (self.energy - 2 * distance)}
                        state.close_note = string.format("turn %d: %d px from boss, stepping to x=%d",
                            state.my_turns[tostring(fight_state.map_id)] or 0, math.abs(boss.x - self.position.x), nx)
                        state.reason = "Stepping away from the boss"
                        return
                    end
                    state.close_note = string.format("turn %d: %d px from boss, no room to step",
                        state.my_turns[tostring(fight_state.map_id)] or 0, math.abs(boss.x - self.position.x))
                end
                if state.step_sent then
                    -- o servidor não devolve a posição a quem andou: espera o passo e mira do destino
                    if now() - state.step_sent.at < step.settle then
                        state.reason = "Waiting for the step to finish"
                        return
                    end
                    local real = self
                    local dest = {x = state.step_sent.x, y = state.step_sent.y}
                    local energy = state.step_sent.energy
                    self = setmetatable({position = dest, destination = false, energy = energy},
                        {__index = function(_, k) return real[k] end})
                end
            end
        end

        local candidates = target_candidates(self, fight_state)
        local shift = stuck_shift(fight_state)
        if shift > 0 then
            candidates = rotated(candidates, shift)
            state.stuck_note = string.format("stage %s turn %d: rotating aim (shift %d, %d targets)",
                tostring(fight_state.map_id), state.my_turns[tostring(fight_state.map_id)] or 0, shift, #candidates)
        end
        if #candidates == 0 then
            state.reason = "Waiting for an alive server target or mission result"
            return
        end

        -- 10/11 (aprovado pelo usuário): derrotas por tempo no estágio 1 com um inimigo grudado nela. O tiro de
        -- perto escolhia sempre o grudado e errava turno após turno, então girar a ordem não mudava nada. Preso
        -- (shift ímpar: turnos 20, 22, 24...), mira só nos inimigos fora do raio do tiro de perto; nos turnos
        -- pares continua o tiro de perto. Sem inimigo longe, segue como antes.
        local close = config.close_kill[tostring(fight_state.map_id)]
        local skip_close = false
        if close and shift % 2 == 1 then
            local far = {}
            for _, candidate in ipairs(candidates) do
                local dx, dy = candidate.position.x - self.position.x, candidate.position.y - self.position.y
                if dx * dx + dy * dy >= close.radius * close.radius then far[#far + 1] = candidate end
            end
            if #far > 0 then
                candidates, skip_close = far, true
                state.stuck_note = (state.stuck_note or "") .. "; aiming at far target"
            end
        end

        local profiles = {false}
        local full_from = config.full_maps[tostring(fight_state.map_id)]
        local full = full_from and (state.my_turns[tostring(fight_state.map_id)] or 0) >= full_from
        local full_trident = false
        if state.trident_requested then
            profiles = {true}
            full_trident = true
        elseif can_plan_trident(self, fight_state) then
            profiles = {true, false}
            if full then profiles = {true}; full_trident = true end
        end
        if full then state.full_note = string.format("turn %d: full (trident=%s)",
            state.my_turns[tostring(fight_state.map_id)] or 0, tostring(full_trident)) end

        local target, solution
        local close_used = false
        if close and not skip_close and self.hp and self.max_hp and self.hp >= self.max_hp * close.min_hp then
            for _, candidate in ipairs(candidates) do
                local dx, dy = candidate.position.x - self.position.x, candidate.position.y - self.position.y
                if dx * dx + dy * dy < close.radius * close.radius then
                    local found, result = pcall(aim, self, candidate, fight_state, nil, false, true)
                    if found and result.direct then
                        target, solution, close_used = candidate, result, true
                        state.close_note = string.format("close kill %s at %d px (hp %d/%d)", tostring(candidate.id),
                            math.floor(math.sqrt(dx * dx + dy * dy)), self.hp, self.max_hp)
                        break
                    end
                    if not found and (type(result) ~= "table" or result.code ~= "formigueiro_no_shot") then error(result, 0) end
                end
            end
        end
        for _, candidate in ipairs(solution and {} or candidates) do
            for _, use_trident in ipairs(profiles) do
                local found, result = pcall(aim, self, candidate, fight_state, nil, use_trident)
                if found then
                    target, solution = candidate, result
                    break
                end
                if type(result) ~= "table" or result.code ~= "formigueiro_no_shot" then error(result, 0) end
            end
            if solution then break end
        end
        -- Alvo único bloqueado (galinhas grudadas na frente): mata primeiro quem bloqueia, com tiro limpo.
        if not solution and config.target_only[tostring(fight_state.map_id)] and not full then
            candidates = target_candidates(self, fight_state, true)
            for _, candidate in ipairs(candidates) do
                local found, result = pcall(aim, self, candidate, fight_state, nil, false)
                if found then
                    target, solution = candidate, result
                    state.blocked_note = "house blocked; clearing " .. tostring(candidate.name)
                    break
                end
                if type(result) ~= "table" or result.code ~= "formigueiro_no_shot" then error(result, 0) end
            end
        end
        if not solution then
            for _, candidate in ipairs(candidates) do
                local found, result = pcall(aim, self, candidate, fight_state, nil, state.trident_requested or full_trident, true)
                if found then
                    if better_shot(result, solution) then
                        target, solution = candidate, result
                    end
                    if result.direct then break end
                elseif type(result) ~= "table" or result.code ~= "formigueiro_no_shot" then
                    error(result, 0)
                end
            end
        end
        -- 13/11: o tiro por cima/por trás sem caminho virava "melhor esforço" com força 1 (não fazia nada): tenta direto no corpo.
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
        end

        state.trident_allowed = solution.trident
        state.attack_mode = "clear_shot"
        if solution.risky and full then
            -- Regra do usuário: do turno 3 em diante bate full sempre, mesmo sem tiro limpo.
            state.attack_mode = solution.direct and "full_shot" or "full_best_effort"
            if not prepare_buffs(self, target, fight_state) then return end
        elseif solution.risky then
            state.attack_mode = "close_shot"
            if not solution.direct then state.attack_mode = "best_effort_shot" end
            state.props_done = true
            state.pow = state.pow or "attack_fallback"
            state.damage_reason = "Attack fallback: no additional buffs"
        elseif not prepare_buffs(self, target, fight_state) then
            return
        end
        local elapsed = math.min(127, math.floor((now() - state.turn_started)))
        send(fight.face, {direction = solution.direction})
        send(fight.shoot_tag, {started = true, shot_type = elapsed})
        send(fight.shoot, solution.shot)
        state.shot_action = state.last_action
        state.shot_followup = false
        state.shot_sent_at = now()
        cronometrar(string.format("%s tiro %.2fs itens=%d", tostring(fight_state.map_id),
            now() - state.turn_started, #(state.confirmed_props or {})))
        -- 11/11: só diagnóstico (não muda a jogada). Derrota por tempo no estágio 1 mesmo com a mira alternada:
        -- guarda os últimos tiros de cada estágio preso (alvo, vida/posição dos inimigos, força/ângulo), que vão
        -- para a nota da derrota, para ver se o tiro no alvo longe sai e se acerta.
        if config.stuck_turns[tostring(fight_state.map_id)] then
            local inimigos = {}
            for _, a in ipairs(fight_state.actors or {}) do
                local hp = a.pending_hp
                if hp == nil then hp = a.hp end
                local p = a.destination or a.position or {}
                if a.team and a.team ~= self.team and a.alive ~= false and hp and hp > 0 then
                    inimigos[#inimigos + 1] = string.format("%s:%s@%s,%s", tostring(a.id), tostring(hp), tostring(p.x), tostring(p.y))
                end
            end
            table.insert(state.shot_log, 1, string.format("[t%d s%d %s%s alvo=%s eu@%s,%s hp=%s f=%s a=%s dir=%s tri=%s direto=%s inim=%s]",
                state.my_turns[tostring(fight_state.map_id)] or 0, shift, state.attack_mode,
                skip_close and "/longe" or (close_used and "/perto" or ""),
                tostring(target.id), tostring(self.position.x), tostring(self.position.y), tostring(self.hp),
                tostring(solution.shot.force), tostring(solution.shot.angle), tostring(solution.direction),
                tostring(solution.trident), tostring(solution.direct), table.concat(inimigos, " ")))
            state.shot_log[13] = nil
        end
        state.fired_turn = key
        state.target_id = target.id
        state.target_position_source = target.position_source
        state.shot = solution.shot
        state.reason = "Waiting for the shot result and next turn"
    end

    local function collect_cards(own_id)
        local cards = fight.cards()
        if not cards.known then return end

        if state.card_revision ~= cards.revision then
            state.card_revision = cards.revision
            state.cards_selected = 0
            state.card_waiting = false
            state.card_finished = false
            state.continued = false
            state.reward_completion = nil
        end

        local result
        for _, entry in ipairs(cards.players) do
            if entry.player_id == own_id then result = entry end
        end
        if not result then return end
        if not result.winner then
            if state.phase ~= "lost" then
                local me = fight.self() or {}
                local turnos = {}
                for mapa, n in pairs(state.my_turns or {}) do turnos[#turnos + 1] = mapa .. ":" .. n end
                table.sort(turnos)
                -- 08/11: derrota por tempo no estágio 1 (64 turnos, ela viva): anota quem sobrou e onde, para achar a causa.
                local vivos = {}
                for _, a in ipairs(fight.state().actors or {}) do
                    local hp = a.pending_hp
                    if hp == nil then hp = a.hp end
                    local p = a.destination or a.position or {}
                    if a.team and a.team ~= me.team and a.alive ~= false and hp and hp > 0 then
                        vivos[#vivos + 1] = string.format("%s:%s@%s,%s", string.gsub(tostring(a.name), "%z", ""), tostring(hp), tostring(p.x), tostring(p.y))
                    end
                end
                local pos = me.position or {}
                table.insert(derrotas, 1, string.format("%.0fs mapa=%s vida=%s/%s viva=%s turnos=%s modo=%s voo=%s andou=%s eu@%s,%s inimigos=%s perto=%s bloqueio=%s travado=%s tiros=%s",
                    now(), tostring(fight.state().map_id), tostring(me.hp), tostring(me.max_hp), tostring(me.alive),
                    table.concat(turnos, ","), tostring(state.attack_mode), tostring(state.fly_note), tostring(state.walk_note),
                    tostring(pos.x), tostring(pos.y), table.concat(vivos, " "), tostring(state.close_note), tostring(state.blocked_note), tostring(state.stuck_note), table.concat(state.shot_log or {}, " ")))
                derrotas[11] = nil
            end
            state.phase = "lost"
            state.reason = "The server reported a lost mission"
            return
        end

        local awarded, normal_awarded = 0, 0
        local expected_items = {}
        -- Gold cards carry template -100 ("Moedas de ouro"): the server credits the money directly,
        -- so it never reaches the bag and is recorded apart, per board.
        local gold = 0
        for _, award in ipairs(cards.awards) do
            local owner = award.player_id
            if cards.type == 1 then
                local actor = fight.actor(award.living_id)
                owner = actor and actor.player_id
            end
            if owner == own_id then
                awarded = awarded + 1
                if award.template_id > 0 and award.count > 0 then
                    expected_items[tostring(award.template_id)] = (expected_items[tostring(award.template_id)] or 0) + award.count
                elseif award.template_id == -100 and award.count > 0 then
                    gold = gold + award.count
                end
                if award.vip ~= true then normal_awarded = normal_awarded + 1 end
                if award.index == state.card_waiting_index then state.card_waiting = false end
            end
        end
        local goal = result.card_count
        -- Extra final cards are only awarded to some characters (VIP); for the others the request
        -- is silently ignored and the board stays open until its timer ends. After one unanswered
        -- extra pick the character stops asking for extras for config.extra_retry_runs runs.
        local extras = (farm.no_extra_until or 0) <= farm.completed_runs
        if cards.type == 2 and extras then goal = math.max(goal, config.final_cards) end
        if state.card_waiting and state.card_waiting_extra_at
            and now() - state.card_waiting_extra_at >= config.extra_card_timeout then
            state.card_waiting = false
            state.card_waiting_extra_at = nil
            farm.no_extra_until = farm.completed_runs + config.extra_retry_runs
            goal = math.max(result.card_count, math.min(goal, awarded))
        elseif not extras then
            goal = math.max(result.card_count, math.min(goal, awarded))
        end
        state.cards_goal = goal
        state.awards = awarded
        state.cards_selected = awarded
        if awarded >= goal or cards.closed then state.card_waiting = false end
        if state.card_waiting then return end

        state.phase = "cards"
        if cards.type ~= 0 and not cards.closed and not cards.finished and not cards.system_selection then
            if awarded < goal then
                if #cards.requested > 0 then return end
                if #cards.available_indices == 0 then return end
                local choices = cards.available_indices
                if party then choices = party:card_indices(cards.available_indices) end
                if #choices == 0 then
                    state.reason = "Waiting for an available card in this character's allocation"
                    return
                end
                local index = choices[math.random(#choices)]
                if cards.type == 1 then
                    send(fight.take_boss_card, {index = index})
                elseif cards.type == 2 then
                    if normal_awarded < result.card_count then
                        send(fight.take_card, {index = index})
                    else
                        send(fight.take_extra_card, {index = index})
                        state.card_waiting_extra_at = now()
                    end
                else
                    pause("Unsupported reward board: " .. tostring(cards.type))
                    return
                end
                state.card_waiting = true
                state.card_waiting_index = index
                state.reason = "Waiting for the selected card award"
                return
            end

            if not state.card_finished then
                if cards.type == 1 then send(fight.finish_boss_cards) else send(fight.finish_cards) end
                state.card_finished = true
                state.reason = "Waiting for card completion"
                return
            end
        end

        if awarded >= goal or cards.closed or cards.type == 0 then
            local collected, reason = state.collection:step(expected_items)
            state.reason = reason
            if not collected then return end
            local delivery_key = fight.state().generation .. ":" .. tostring(fight.state().map_id) .. ":" .. cards.revision
            if not farm.deliveries[delivery_key] then
                farm.deliveries[delivery_key] = state.collection:status()
            end
            local delivery = farm.deliveries[delivery_key]
            if gold > ((delivery.received or {})["-100"] or 0) then
                delivery.received = delivery.received or {}
                delivery.received["-100"] = gold
            end
        end

        if cards.has_next_mission then
            if awarded < goal and not cards.closed then return end
            if not cards.can_enter_final and (not party or party:is_leader()) then
                state.reason = "Waiting for server permission for the final stage"
                return
            end
            if party then
                local stages_done = state.stage_count
                for index, map_id in ipairs(config.stage_order) do
                    if map_id == fight.state().map_id then stages_done = index end
                end
                local ready, reason = party:prepare_mission(room.state(), stages_done)
                state.reason = reason
                if not ready then return end
            end
            if not state.continued then
                send(fight.mission_start, {flag = true})
                state.continued = true
            end
            state.phase = "continuing"
            state.reason = "Waiting for the next mission"
        elseif awarded >= goal and fight.state().phase == "ended" then
            state.phase = "done"
            state.reward_completion = "server_victory_and_inventory_delivery"
            state.reason = "The server confirmed victory and permanent-inventory delivery"
        elseif cards.closed or cards.type == 0 then
            state.phase = "done"
            state.reward_completion = "server_board_closed_and_inventory_delivery"
            state.reason = "The run and permanent-inventory delivery are complete"
            if awarded < goal then
                -- In a party the server may close the board before a slower member's pick is awarded;
                -- nothing more can arrive, so the run counts as done and the shortfall is recorded.
                state.cards_shortfall = goal - awarded
                state.reason = "The server closed the board with " .. tostring(goal - awarded) .. " card(s) not awarded"
            end
        else
            state.phase = "finishing"
            state.reason = "Waiting for the final card reveal"
        end
    end

    local function repeat_run(room_state, fight_state)
        if state.phase ~= "done" and state.phase ~= "lost" then return false end
        if fight_state.phase ~= "ended" then
            state.reason = "Waiting for the server to confirm the run ended"
            return true
        end

        local collected, reason = state.collection:step({})
        if not collected then
            state.reason = reason
            return true
        end

        if not state.finished_phase then
            state.finished_phase = state.phase
            farm.completed_runs = farm.completed_runs + 1
            if state.phase == "done" then
                farm.victories = farm.victories + 1
            else
                farm.defeats = farm.defeats + 1
            end
        end

        local cards = fight.cards()
        if room_state.id and state.phase == "done" and cards.known and not cards.closed then
            state.reason = "Rewards received; waiting for the server to close the board before reusing the room"
            return true
        end

        if farm.stop_after and farm.completed_runs >= farm.stop_after then
            state.phase = "stopped"
            state.reason = "Stopped after the run with permanent-inventory delivery confirmed"
            return true
        end

        -- Convidada: terminada a partida (cartas e mochila), sai da sala; a principal recomeça sozinha.
        state = new_run()
        state.room_id = room_state.id
        state.restart_generation = fight_state.generation
        state.reason = "Preparing the next run in the current room"
        return true
    end

    local function recover_room_loss(room_state, fight_state)
        if not state.room_id or room_state.id == state.room_id then return false end
        if room_state.id or state.fight_started or fight_state.phase ~= "unknown" then
            pause("The room changed during this run")
            return true
        end

        local recoveries = state.room_recoveries + 1
        if recoveries > 3 then
            pause("The server removed three rooms before a fight could start")
            return true
        end

        state = new_run()
        state.room_recoveries = recoveries
        state.retry_at = now() + 0.25 * recoveries
        state.reason = "Server confirmed room exit before the fight; recreating the room"
        return true
    end

    local function start_wait_expired(room_state)
        if state.phase ~= "starting" or not state.start_requested_at then return false end
        if room_state.start_cancelled_revision ~= state.start_cancelled_revision then
            state.phase = "start_cancelled"
            state.reason = "The server cancelled the fight start"
            return true
        end
        if now() - state.start_requested_at >= config.start_timeout then
            state.phase = "start_unconfirmed"
            state.reason = "No server fight start within 15 seconds; the sent request was not retried"
            return true
        end
        return false
    end

    local function verification_pending()
        if not captcha then return state.phase == "verification" end

        local verification = captcha.state()
        if verification.required then
            if state.phase ~= "verification" then state.before_verification = state.phase end
            state.phase = "verification"
            if state.captcha_revision ~= verification.revision then
                state.captcha_revision = verification.revision
                state.captcha_action = nil
                state.captcha_rechecks = 0
                state.captcha_submitted_at = nil
                state.captcha_issue = nil
            end
            if not verification.known then
                state.reason = "Server verification is required, but the image could not be decoded"
                return true
            end
            if state.captcha_issue then
                state.reason = state.captcha_issue
                return true
            end
            if state.captcha_action then
                local receipt = actions.get(state.captcha_action)
                -- Both reasons mean the answer was never written, so resending the same answer is safe.
                local unwritten = receipt.reason == "stale_state" or receipt.reason == "packet_boundary_unavailable"
                if receipt.status == "rejected" and unwritten and state.captcha_rechecks < 30 then
                    state.captcha_rechecks = state.captcha_rechecks + 1
                    state.captcha_action = nil
                    state.captcha_retry_at = now() + math.max(config.retry_delay, 0.5)
                    return true
                end
                if receipt.status == "uncertain" then
                    pause("CAPTCHA answer has an uncertain write outcome")
                    return true
                end
                if receipt.status == "rejected" or receipt.status == "cancelled" then
                    state.captcha_issue = "CAPTCHA answer was not written: " .. receipt.reason
                    state.reason = state.captcha_issue
                    return true
                end
                state.reason = "CAPTCHA answer " .. receipt.status .. "; waiting for the server response"
                return true
            end
            if state.captcha_retry_at and now() < state.captcha_retry_at then return true end
            local solution = captcha.solve()
            state.captcha_solver = solution.status
            state.captcha_confidence = solution.confidence
            if solution.status == "pending" then
                state.reason = "Recognizing the current CAPTCHA image"
                return true
            end
            if solution.status ~= "solved" then
                state.reason = solution.reason
                return true
            end
            if solution.revision ~= verification.revision then return true end
            state.captcha_action = captcha.answer({revision = solution.revision, text = solution.text})
            state.captcha_submitted_at = now()
            state.reason = "Submitting the CAPTCHA answer"
            return true
        end
        if state.phase == "verification" then
            if not verification.known then return true end
            state.phase = state.before_verification
            state.before_verification = nil
            state.captcha_submitted_at = nil
            if state.start_cancelled then
                state.phase = "home"
                state.configured = false
                state.start_requested_at = nil
                state.start_cancelled = nil
                state.setup_rechecks = 0
            end
        end
        return false
    end

    local function advance()
        if state.phase == "stopped" then return end

        if not session.info().valid or not session.info().can_send then
            pause("Session is not writable")
            return
        end
        if verification_pending() then return end
        if not player.state().id or not writes_complete() then return end
        if party and not party:settle() then return end

        if state.retry_at and now() < state.retry_at then return end
        state.retry_at = nil

        local room_state = room.state()
        local fight_state = fight.state()
        state.collection:observe()
        if repeat_run(room_state, fight_state) then return end
        if not solo(room_state, player.state().id) then return end
        for _, actor in ipairs(fight_state.actors) do
            if not party and actor.player_id and actor.player_id ~= player.state().id then
                pause("Another player is in the fight; this script requires a solo run")
                return
            end
        end

        if recover_room_loss(room_state, fight_state) then return end
        if state.restart_generation == fight_state.generation and fight_state.phase == "ended" then
            if not start_wait_expired(room_state) then prepare_room(room_state) end
            return
        end

        if fight_state.phase == "loading" or fight_state.phase == "active" then
            if not supported_maps[tostring(fight_state.map_id)] then
                pause("Unsupported fight map: " .. tostring(fight_state.map_id))
                return
            end
            state.phase = "fighting"
            if state.collection_map and state.collection_map ~= fight_state.map_id then
                if not state.collection.complete then
                    pause("The next mission arrived before permanent-inventory delivery was confirmed")
                    return
                end
                state.collection = rewards.new()
            end
            state.collection_map = fight_state.map_id
            state.fight_started = true
            state.restart_generation = nil
            state.start_requested_at = nil
            state.map_id = fight_state.map_id
            if not state.stages_seen[tostring(fight_state.map_id)] then
                state.stages_seen[tostring(fight_state.map_id)] = true
                state.stage_count = state.stage_count + 1
            end
            if fight_state.phase == "loading" then
                local key = fight_state.generation .. ":" .. fight_state.map_id
                if not state.loaded[key] then
                    state.loading_generation = fight_state.generation
                    state.loading_map = fight_state.map_id
                    send(fight.load_progress, {progress = 100})
                    state.loaded[key] = true
                end
                state.reason = "Waiting for stage loading"
                return
            end
            attack(fight_state)
        elseif fight_state.phase == "mission_ended" or fight_state.phase == "ended" then
            collect_cards(player.state().id)
        elseif fight_state.phase == "created" then
            state.fight_started = true
            state.restart_generation = nil
            state.start_requested_at = nil
            state.phase = "starting"
        else
            if not start_wait_expired(room_state) then prepare_room(room_state) end
        end
    end

    function olimpiadas_stop_after_run()
        farm.stop_after = farm.completed_runs + 1
        return farm.stop_after
    end

    local function undelivered_runs(field)
        local count = 0
        for _, delivery in pairs(farm.deliveries) do
            if delivery[field] then count = count + 1 end
        end
        return count
    end

    function olimpiadas_status()
        local shot
        if state.shot then
            shot = {x = state.shot.x, y = state.shot.y, force = state.shot.force, angle = state.shot.angle}
        end
        return {
            phase = state.phase, reason = state.reason, room_id = state.room_id, map_id = state.map_id,
            last_action = state.last_action, target_id = state.target_id, shot = shot, awards = state.awards,
            setup_rechecks = state.setup_rechecks, room_recoveries = state.room_recoveries,
            target_position_source = state.target_position_source,
            reactions = reacoes, defeat_notes = derrotas,
            pow = state.pow, damage_buff = state.damage_buff, damage_reason = state.damage_reason,
            confirmed_props = state.confirmed_props,
            buff_message = state.buff_message, buff_issue = state.buff_issue,
            wind_used = state.wind_used, wind_fallback = state.wind_fallback,
            player_position_source = state.player_position_source,
            cards_selected = state.cards_selected, cards_goal = state.cards_goal,
            reward_completion = state.reward_completion,
            loot = state.collection:status(),
            preflight = state.preflight:status(),
            deliveries = farm.deliveries,
            undelivered_runs = undelivered_runs("undelivered"),
            mailed_runs = undelivered_runs("sent_to_mail"),
            party = party and party:status(),
            attack_mode = state.attack_mode,
            balloon_note = state.balloon_note, balloon_log = state.balloon_log, hp_log = state.hp_log, shot_log = state.shot_log, balloons = #balloon_list(), my_turns = state.my_turns, fly_note = state.fly_note, walk_note = state.walk_note, close_note = state.close_note, full_note = state.full_note, flown = state.flown, stage_count = state.stage_count, blocked_note = state.blocked_note,
            attack_policy = config.attack_policy,
            setup_wait_seconds = state.setup_requested_at and now() - state.setup_requested_at,
            start_wait_seconds = state.start_requested_at and now() - state.start_requested_at,
            captcha_revision = state.captcha_revision,
            captcha_solver = state.captcha_solver, captcha_confidence = state.captcha_confidence,
            captcha_wait_seconds = state.captcha_submitted_at and now() - state.captcha_submitted_at,
            farming = state.phase ~= "stopped", next_run_number = farm.completed_runs + 1,
            completed_runs = farm.completed_runs, victories = farm.victories, defeats = farm.defeats,
        }
    end

    local function progress(packet)
        if state.phase == "paused" or state.phase == "incomplete" then return end

        local ok, problem = pcall(function()
            if packet then
                if party then party:observe(packet) end
                observe_buffs(packet)
            end
            check_buff_wait(packet)
            if state.phase ~= "paused" then advance() end
        end)
        if not ok then
            if type(problem) == "table" and problem.code == "formigueiro_aim_pending" then
                state.reason = problem.message
                return
            end
            if type(problem) == "table" then
                pause(tostring(problem.code) .. ": " .. tostring(problem.message))
            else
                pause(tostring(problem))
            end
        end
    end

    local function on_packet(packet)
        if packet then pcall(observe_balloons, packet); pcall(observe_hp) end
        if state.phase == "incomplete" or state.phase == "paused" then return end
        if packet.direction ~= "ServerToClient" then return end
        if packet.code == 200 and not captcha then
            state.phase = "verification"
            state.reason = "Server verification received; restart Bot and Relay to enable the CAPTCHA API"
            return
        end
        if packet.code == 94 and packet.status == "decoded" and packet.fields.subcommand == 11
            and state.start_requested_at then
            state.start_cancelled = true
            if state.phase == "starting" then
                state.phase = "start_cancelled"
                state.reason = "The server cancelled the fight start"
            end
        end
        if state.phase == "done" or state.phase == "lost" then
            progress(packet)
            return
        end
        if packet.code == 91 and packet.status == "decoded" then
            local self = fight.self()
            local current = fight.state()
            local key = current.generation .. ":" .. current.turn_revision
            if packet.fields.subcommand == 2 and self and packet.fields.living_id == self.id
                and state.fired_turn == key then
                state.server_shots = (state.server_shots or 0) + 1
                state.max_shots = math.max(state.max_shots or 1, self.shoot_count or 1)
                if state.server_shots >= math.max(2, state.max_shots) then
                    state.followup_sent = true
                elseif not state.followup_sent and (state.followups or 0) < state.server_shots then
                    state.followup_started = now()
                end
            end
            if packet.fields.subcommand == 98 and packet.fields.card_index == 50
                and packet.client_id == player.state().id then
                state.card_waiting = false
                state.phase = "incomplete"
                state.reason = "The server rejected a card selection without an award"
                return
            end
        end
        if packet.status == "decoded" and packet.code == 94 and packet.fields.subcommand == 2
            and (state.phase == "configuring" or state.phase == "configuration_unconfirmed") then
            state.phase = "configuring"
            state.configured = true
            state.setup_requested_at = nil
        end

        progress(packet)
    end

    report_status()

    return {
        api_version = 1,
        on_tick = function()
            progress()
            report_status()
        end,
        on_action = function()
            progress()
            report_status()
        end,
        on_packet = function(packet)
            on_packet(packet)
            report_status()
        end,
        on_close = function(event)
            pause("Session closed: " .. tostring(event.reason))
            report_status()
        end,
    }
end

return M
