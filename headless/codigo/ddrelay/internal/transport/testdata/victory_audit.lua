local original = hooks.on_packet
local turns, fires, buffs, reports = 0, 0, 0, 0
local wind_mismatches = 0
local turn_wind
local shots = {}
local last_shot

function solo_audit()
    local status = formigueiro_status()
    return {
        turns = turns, fires = fires, buffs = buffs, reports = reports,
        wind_mismatches = wind_mismatches, shots = shots,
        phase = status.phase, reason = status.reason,
        awards = status.awards, cards_goal = status.cards_goal,
    }
end

hooks.on_packet = function(packet)
    original(packet)
    local status = formigueiro_status()
    local self = fight.self()
    local current = fight.state()
    if packet.direction == "ServerToClient" and packet.code == 91 then
        if packet.fields.subcommand == 6 then
            turn_wind = fight.wind().tenths
            if self and current.turn and current.turn.actor_id == self.id then turns = turns + 1 end
        elseif packet.fields.subcommand == 2 then
            fires = fires + 1
            if turn_wind ~= packet.fields.wind_tenths then wind_mismatches = wind_mismatches + 1 end
        elseif packet.fields.subcommand == 32 and status.damage_buff == packet.fields.template_id then
            buffs = buffs + 1
        end
    end
    if packet.direction == "ClientToServer" and packet.code == 91 and packet.fields.subcommand == 9
        and status.player_position_source == "client_report" then reports = reports + 1 end
    if status.last_action and status.last_action ~= last_shot and actions.get(status.last_action).name == "shoot" then
        last_shot = status.last_action
        shots[#shots + 1] = {
            map = current.map_id, wind = status.wind_used, fallback = status.wind_fallback,
            force = status.shot.force, angle = status.shot.angle,
        }
    end
end

return hooks
