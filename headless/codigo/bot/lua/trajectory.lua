local catalog = require("catalog")
local terrain = require("terrain")

local M = {}
local dt = 0.04
local force_max = 2000
local laser_projectile_id = 87

local function fail(code, message)
    error({code = code, message = message}, 0)
end

local function fields(value, allowed)
    if type(value) ~= "table" then
        fail("trajectory_argument", "expected an argument table")
    end
    for key in pairs(value) do
        if not allowed[key] then
            fail("trajectory_argument", "unknown argument: " .. tostring(key))
        end
    end
end

local function number(value, name, minimum, maximum)
    if type(value) ~= "number" or value ~= value or value < minimum or value > maximum then
        fail("trajectory_argument", name .. " is outside its finite range")
    end
    return value
end

local function integer(value, name, minimum, maximum)
    number(value, name, minimum, maximum)
    if value ~= math.floor(value) then
        fail("trajectory_argument", name .. " must be an integer")
    end
    return value
end

local function trunc(value)
    if value < 0 then return math.ceil(value) end
    return math.floor(value)
end

local function shot_copy(shot)
    return {x = shot.x, y = shot.y, force = shot.force, angle = shot.angle}
end

function M.model(args)
    fields(args, {map_id = true, projectile_id = true, wind = true})
    number(args.wind, "wind", -1000, 1000)
    if args.projectile_id == laser_projectile_id then
        fail("trajectory_unsupported", "laser projectiles use SkillBomb")
    end
    local map = catalog.map({id = args.map_id})
    local ball = catalog.projectile({id = args.projectile_id})
    return {
        mass = ball.mass,
        drag = map.drag * ball.drag,
        gravity_force = map.gravity * ball.weight * ball.mass,
        wind_force = args.wind * ball.wind * map.wind_rate,
        dt = dt,
        map_id = map.id,
        width = map.width,
        height = map.height
    }
end

function M.step(state, model)
    local vx = state.vx + (model.wind_force - model.drag * state.vx) / model.mass * dt
    local vy = state.vy + (model.gravity_force - model.drag * state.vy) / model.mass * dt
    return {x = state.x + vx * dt, y = state.y + vy * dt, vx = vx, vy = vy}
end

function M.slope(args)
    fields(args, {map_id = true, x = true, y = true})
    local x = number(args.x, "x", -1000000, 1000000)
    local y = number(args.y, "y", -1000000, 1000000)
    local sides = {}
    for _, direction in ipairs({1, -1}) do
        local sum_x, sum_y, count = x, y, 1
        for offset = 1, 16, 2 do
            for vertical = -10, 10 do
                local px, py = x + direction * offset, y - vertical
                if not terrain.solid({map_id = args.map_id, x = px, y = py}) then
                    if vertical ~= -10 then
                        sum_x, sum_y, count = sum_x + px, sum_y + py, count + 1
                    end
                    break
                end
            end
        end
        sides[#sides + 1] = {x = sum_x / count, y = sum_y / count}
    end
    return math.floor(math.deg(math.atan2(sides[1].y - sides[2].y, sides[1].x - sides[2].x)))
end

function M.launch(args)
    fields(args, {x = true, y = true, direction = true, slope = true, gun_angle = true, force = true})
    local x = number(args.x, "x", -1000000, 1000000)
    local y = number(args.y, "y", -1000000, 1000000)
    local slope = number(args.slope, "slope", -180, 180)
    local gun_angle = number(args.gun_angle, "gun_angle", -180, 180)
    local force = integer(args.force, "force", 0, force_max)
    if args.direction ~= -1 and args.direction ~= 1 then
        fail("trajectory_argument", "direction must be -1 or 1")
    end
    local angle = slope - gun_angle
    if args.direction < 0 then angle = slope + gun_angle - 180 end
    local radians = math.rad(slope)
    return {
        x = trunc(x + 30 * args.direction * math.cos(radians) + 20 * math.sin(radians)),
        y = trunc(y - 3 + 30 * args.direction * math.sin(radians) - 20 * math.cos(radians)),
        force = force,
        angle = trunc(angle)
    }
end

local function target_copy(target, model)
    fields(target, {x = true, y = true, radius = true})
    return {
        x = number(target.x, "target.x", 0, model.width - 1),
        y = number(target.y, "target.y", 0, model.height - 1),
        radius = number(target.radius, "target.radius", 0.01, 100)
    }
end

local function simulate(model, shot, target, max_steps)
    local radians = math.rad(shot.angle)
    local x, y = shot.x, shot.y
    local vx, vy = trunc(shot.force * math.cos(radians)), trunc(shot.force * math.sin(radians))
    local closest = math.huge
    local function result(status, time)
        return {status = status, x = x, y = y, time = time, distance = closest}
    end
    for step = 1, max_steps do
        vx = vx + (model.wind_force - model.drag * vx) / model.mass * dt
        vy = vy + (model.gravity_force - model.drag * vy) / model.mass * dt
        local dx, dy = vx * dt, vy * dt
        local contact = terrain.trace({map_id = model.map_id, x = x, y = y, to_x = x + dx, to_y = y + dy})
        local length_squared = dx * dx + dy * dy
        local projection = 0
        if length_squared > 0 then
            projection = math.max(0, math.min(contact.fraction, ((target.x - x) * dx + (target.y - y) * dy) / length_squared))
        end
        local distance = math.sqrt((x + projection * dx - target.x)^2 + (y + projection * dy - target.y)^2)
        closest = math.min(closest, distance)
        if distance <= target.radius then
            x, y = x + projection * dx, y + projection * dy
            return result("target", (step - 1 + projection) * dt)
        end
        x, y = contact.x, contact.y
        if contact.hit then return result("terrain", (step - 1 + contact.fraction) * dt) end
        if x < 0 or x > model.width or y > model.height then return result("bounds", step * dt) end
    end
    return result("limit", max_steps * dt)
end

function M.simulate(args)
    fields(args, {map_id = true, projectile_id = true, wind = true, shot = true, target = true, max_steps = true})
    fields(args.shot, {x = true, y = true, force = true, angle = true})
    local shot = {
        x = integer(args.shot.x, "shot.x", -1000000, 1000000),
        y = integer(args.shot.y, "shot.y", -1000000, 1000000),
        force = integer(args.shot.force, "force", 0, force_max),
        angle = integer(args.shot.angle, "angle", -540, 540)
    }
    local model = M.model({map_id = args.map_id, projectile_id = args.projectile_id, wind = args.wind})
    return simulate(model, shot, target_copy(args.target, model), integer(args.max_steps, "max_steps", 1, 750))
end

local function current_context(actor_id, code)
    local state = fight.state()
    local actor = fight.actor(actor_id)
    local wind = fight.wind()
    local player_id = player.state().id
    if not session.info().valid or state.phase ~= "active" or not state.turn or state.turn.actor_id ~= actor_id or
       not actor or not actor.player_id or actor.player_id ~= player_id or not actor.weapon_id or
       not actor.position or actor.destination or actor.alive == false or (actor.hp and actor.hp <= 0) or
       not state.map_id or not wind.current or wind.value == nil then
        fail(code, "own turn, position, weapon and current wind must be known")
    end
    return {
        generation = state.generation, turn_revision = state.turn_revision, turn_number = state.turn.number,
        actor_id = actor_id, map_id = state.map_id, weapon_id = actor.weapon_id, ball_id = actor.ball_id,
        x = actor.position.x, y = actor.position.y, direction = actor.direction, wind = wind.value
    }
end

local function validate_context(expected)
    local current = current_context(expected.actor_id, "trajectory_stale")
    for _, key in ipairs({"generation", "turn_revision", "turn_number", "actor_id", "map_id", "weapon_id", "ball_id", "x", "y", "direction", "wind"}) do
        if expected[key] ~= current[key] then
            fail("trajectory_stale", "search input changed: " .. key)
        end
    end
end

function M.search(args)
    fields(args, {
        actor_id = true, direction = true, target = true, force_min = true, force_max = true, force_step = true,
        angle_min = true, angle_max = true, angle_step = true, max_steps = true
    })
    local context = current_context(integer(args.actor_id, "actor_id", 0, 2147483647), "trajectory_unavailable")
    local weapon = catalog.weapon({id = context.weapon_id})
    local projectile_id = context.ball_id or weapon.common
    local model = M.model({map_id = context.map_id, projectile_id = projectile_id, wind = context.wind})
    local target = target_copy(args.target, model)
    local min_force = integer(args.force_min, "force_min", 0, force_max)
    local max_force = integer(args.force_max, "force_max", min_force, force_max)
    local force_step = integer(args.force_step, "force_step", 1, force_max)
    local min_angle = integer(args.angle_min, "angle_min", math.ceil(weapon.min_angle), math.floor(weapon.max_angle))
    local max_angle = integer(args.angle_max, "angle_max", min_angle, math.floor(weapon.max_angle))
    local angle_step = integer(args.angle_step, "angle_step", 1, 180)
    local max_steps = integer(args.max_steps, "max_steps", 1, 750)
    local slope = M.slope({map_id = context.map_id, x = context.x, y = context.y})
    local direction = args.direction
    if direction ~= -1 and direction ~= 1 then fail("trajectory_argument", "direction must be -1 or 1") end
    local angle, force, tested = min_angle, min_force, 0
    local solution, impact
    local closest = math.huge
    local job = {}

    function job:step(count)
        validate_context(context)
        integer(count, "batch size", 1, 32)
        for _ = 1, count do
            if solution then break end
            if angle > max_angle then
                fail("trajectory_no_solution", "no unobstructed target intersection in the requested search grid; closest distance " .. tostring(closest))
            end
            local shot = M.launch({x = context.x, y = context.y, direction = direction, slope = slope, gun_angle = angle, force = force})
            local predicted = simulate(model, shot, target, max_steps)
            tested = tested + 1
            closest = math.min(closest, predicted.distance)
            if predicted.status == "target" then solution, impact = shot, predicted end
            force = force + force_step
            if force > max_force then force, angle = min_force, angle + angle_step end
        end
        if solution then
            return {status = "found", tested = tested, direction = direction, shot = shot_copy(solution), distance = impact.distance, time = impact.time}
        end
        return {status = "searching", tested = tested, closest_distance = closest}
    end

    function job:shot()
        validate_context(context)
        if not solution then fail("trajectory_unavailable", "search has no solution yet") end
        return shot_copy(solution)
    end

    return job
end

return M
