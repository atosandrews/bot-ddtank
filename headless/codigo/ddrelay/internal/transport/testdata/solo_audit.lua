local turns = {}
local original = hooks.on_packet

function solo_audit()
    local status = formigueiro_status()
    return {turns = turns, phase = status.phase, reason = status.reason}
end

hooks.on_packet = function(packet)
    original(packet)

    local self = fight.self()
    local current = fight.state()
    if packet.direction ~= "ServerToClient" or packet.code ~= 91 or packet.fields.subcommand ~= 6
        or not self or not current.turn or current.turn.actor_id ~= self.id then
        return
    end

    turns[#turns + 1] = {
        wind = fight.wind().current,
        position = self.position ~= nil,
        moving = self.destination ~= nil,
        weapon = self.weapon_id ~= nil,
        team = self.team ~= nil,
        target_source = formigueiro_status().target_position_source,
    }
end

return hooks
