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

    require("data.projectiles")
    require("data.weapons")

    local config = {
        room_name = "Formigueiro solo",
        password = "solo",
        map_id = 2,
        difficulty = 1,
        aim_height = 18,
        aim_radius = 12,
        max_steps = 300,
        angles = {25, 35, 45, 60, 75},
        buff_timeout = 3,
        fallback_wind = 0,
        final_cards = 3,
        extra_card_timeout = 3,
        extra_retry_runs = 10,
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
            assert(config[key] ~= nil, "Unknown Formigueiro option: " .. key)
            config[key] = value
        end
    end

    for _, map_id in ipairs({1120, 1121}) do
        terrain.solid({map_id = map_id, x = 0, y = 0})
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
        }
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
        if #reason > 1024 then reason = "Status detail exceeds the observation limit; inspect formigueiro_status()" end
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
                skip_buff("Optional buff was not sent: " .. result.status)
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
            state.reason = "Waiting for Formigueiro / Normal confirmation"
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
                pause("Server room configuration does not match Formigueiro / Normal")
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
            minimum_clearance, tostring(use_trident == true), tostring(allow_risk == true),
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

        for _, preferred in ipairs(config.angles) do
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

    local function target_candidates(self, fight_state)
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
                    position_source = source,
                }
            end
        end

        table.sort(candidates, function(a, b)
            if fight_state.map_id == 1121 and (a.max_hp or 0) ~= (b.max_hp or 0) then
                return (a.max_hp or 0) > (b.max_hp or 0)
            end

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
        if fight_state.map_id == 1120 then primary = trident end
        local cost = tonumber(primary.properties[4])
        if energy >= cost and (primary.id ~= trident.id or (state.trident_allowed and not self.special)) then
            plan[#plan + 1] = primary
            energy = energy - cost
        end
        for _, prop in ipairs(damage_props) do
            cost = tonumber(prop.properties[4])
            while energy >= cost do
                plan[#plan + 1] = prop
                energy = energy - cost
                if fight_state.map_id == 1120 then return plan end
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
            elseif fight_state.map_id == 1120 and not state.trident_allowed then
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
        if fight_state.map_id ~= 1120 or self.special then return false end
        if state.props_done or state.prop_plan then return false end

        local controls = self.controls or {}
        if not controls.known or not controls.right_prop_enabled then return false end
        if controls.lock_right_prop or controls.lock_state or controls.frozen then return false end

        return (self.energy or 0) >= tonumber(trident.properties[4])
    end

    local function attack(fight_state)
        state.search_deadline = now() + 0.09  -- 90 ms de cada tick de 100 ms (era 35 ms; 05/11/2026)
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
        if config.attack_policy == "leader" and not party:is_leader() then
            if state.passed_turn ~= key then
                send(fight.pass, {argument = 0})
                state.passed_turn = key
            end
            state.attack_mode = "party_pass"
            state.reason = "Waiting for the server to end the follower turn"
            return
        end

        if state.turn_key ~= key then
            state.turn_key = key
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
            state.shot_followup = false
            state.trident_requested = false
            state.trident_allowed = false
        end
        if state.fired_turn == key then
            if state.followup_started and not state.followup_sent and (self.shoot_count or 1) >= 2
                and (now() - state.followup_started) >= 1 then
                send(fight.shoot, state.shot)
                state.shot_action = state.last_action
                state.shot_followup = true
                state.followup_sent = true
                state.reason = "Waiting for the additional shot result"
            end
            return
        end

        state.player_position_source = "server_position"
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

        local candidates = target_candidates(self, fight_state)
        if #candidates == 0 then
            state.reason = "Waiting for an alive server target or mission result"
            return
        end

        local profiles = {false}
        if state.trident_requested then
            profiles = {true}
        elseif can_plan_trident(self, fight_state) then
            profiles = {true, false}
        end

        local target, solution
        for _, candidate in ipairs(candidates) do
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
        if not solution then
            for _, candidate in ipairs(candidates) do
                local found, result = pcall(aim, self, candidate, fight_state, nil, state.trident_requested, true)
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
        if not solution then
            error({code = "formigueiro_no_shot", message = "No trajectory could be calculated from the server position"}, 0)
        end

        state.trident_allowed = solution.trident
        state.attack_mode = "clear_shot"
        if solution.risky then
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
                local ready, reason = party:prepare_mission(room.state())
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
            if fight_state.map_id ~= 1120 and fight_state.map_id ~= 1121 then
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

    function formigueiro_stop_after_run()
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

    function formigueiro_status()
        local shot
        if state.shot then
            shot = {x = state.shot.x, y = state.shot.y, force = state.shot.force, angle = state.shot.angle}
        end
        return {
            phase = state.phase, reason = state.reason, room_id = state.room_id, map_id = state.map_id,
            last_action = state.last_action, target_id = state.target_id, shot = shot, awards = state.awards,
            setup_rechecks = state.setup_rechecks, room_recoveries = state.room_recoveries,
            target_position_source = state.target_position_source,
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
                if state.server_shots >= 2 then
                    state.followup_sent = true
                elseif not state.followup_started and not state.followup_sent then
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
