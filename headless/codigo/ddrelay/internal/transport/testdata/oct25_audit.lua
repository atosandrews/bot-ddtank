local turns = {}
local original = hooks.on_packet
local fires = {}
local turn_wind

function solo_audit()
    local status = formigueiro_status()
    return {turns = turns, fires = fires, phase = status.phase, reason = status.reason}
end

hooks.on_packet = function(packet)
    original(packet)
    if packet.direction == "ServerToClient" and packet.code == 91 and packet.fields.subcommand == 2 then
        fires[#fires + 1] = {display = turn_wind, reported = packet.fields.wind_tenths}
    end
    local self = fight.self()
    local current = fight.state()
    if packet.direction == "ServerToClient" and packet.code == 91 and packet.fields.subcommand == 6
        and self and current.turn and current.turn.actor_id == self.id then
        turn_wind = fight.wind().tenths
        turns[#turns + 1] = {
            time = packet.time, wind = fight.wind().current, position = self.position ~= nil,
            moving = self.destination ~= nil, weapon = self.weapon_id ~= nil, team = self.team ~= nil,
        }
    end
end
return hooks
