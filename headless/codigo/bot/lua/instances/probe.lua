local experiments = {
    function(entry, state, own)
        entry.collect = session.collect_rewards()
    end,
    function(entry, state, own)
        entry.finish = fight.finish_cards()
    end,
    function(entry, state, own)
        entry.take1 = fight.take_card({index = 1})
        entry.take2 = fight.take_card({index = 2})
        entry.take3 = fight.take_card({index = 99})
    end,
    function(entry, state, own)
        entry.tag = fight.shoot_tag({started = true, shot_type = 1})
        entry.shot1 = fight.shoot({x = own.position.x - 30, y = own.position.y - 23, force = 600, angle = -135})
        entry.shot2 = fight.shoot({x = own.position.x - 30, y = own.position.y - 23, force = 700, angle = -140})
        entry.shot3 = fight.shoot({x = own.position.x - 30, y = own.position.y - 23, force = 800, angle = -145})
    end,
    function(entry, state, own)
        entry.prop3 = fight.use_prop({bag_type = 3, place = 0, template_id = 10022})
        entry.propneg = fight.use_prop({bag_type = 1, place = -5, template_id = 10022})
        entry.propbig = fight.use_prop({bag_type = 1, place = -1, template_id = 999999})
    end,
    function(entry, state, own)
        entry.forged = fight.shoot({x = 52, y = 506, force = 600, angle = -135})
    end,
}

local function step()
    local state = fight.state()
    if state.phase ~= "active" or not state.turn or state.turn.actor_id ~= 0 then return end
    local own = fight.self()
    if not own or not own.alive or not own.position then return end
    if probe == nil then probe = {turns = {}} end
    if probe.turn == state.turn.number then return end
    probe.turn = state.turn.number
    local entry = {turn = state.turn.number, hp = own.hp}
    probe.turns[#probe.turns + 1] = entry
    local experiment = experiments[#probe.turns]
    if not experiment then return end
    entry.name = "experiment_" .. tostring(#probe.turns)
    pcall(experiment, entry, state, own)
end

return {
    api_version = 1,
    on_event = function(event)
        if event.name == "turn_started" then pcall(step) end
    end,
    on_tick = function() pcall(step) end,
}
