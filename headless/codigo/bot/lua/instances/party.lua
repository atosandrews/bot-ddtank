local M = {}
local Party = {}
Party.__index = Party

local function fail(code, message)
    error({code = code, message = message}, 0)
end

function M.new(options, config)
    local own_id = player.state().id
    local members = {}
    local ordinal
    for index, id in ipairs(options.members) do
        if members[id] then fail("party_config", "Repeated party member") end
        members[id] = true
        if id == own_id then ordinal = index - 1 end
    end
    if not ordinal or not members[options.leader_id] or #options.members ~= 4 then
        fail("party_config", "This script requires the four configured characters")
    end

    return setmetatable({
        own_id = own_id,
        leader_id = options.leader_id,
        members = members,
        order = options.members,
        ordinal = ordinal,
        config = config,
        room_states = {},
        mission_states = {},
        sent = {},
        attempts = {},
        setup_revision = 0,
        mission_revision = 0,
    }, Party)
end

function Party:is_leader()
    return self.own_id == self.leader_id
end

function Party:check_members(current)
    for _, member in ipairs(current.players) do
        if not self.members[member.id] then
            fail("party_member_unknown", "The room contains an unconfigured player: " .. member.id)
        end
    end
    return true
end

function Party:settle()
    if self.retry_at and session.now() < self.retry_at then return false end
    if not self.pending then return true end

    local receipt = actions.get(self.pending.id)
    if not receipt then fail("party_receipt_missing", "Party command receipt is unavailable") end
    if receipt.status == "queued" or receipt.status == "dispatched" then return false end
    local key = self.pending.key
    if receipt.status == "rejected" and receipt.reason == "stale_state" and self.attempts[key] < 5 then
        self.pending = nil
        self.retry_at = session.now() + 0.1
        return false
    end
    if receipt.status ~= "sent" then
        fail("party_command_failed", receipt.name .. ": " .. receipt.status .. ": " .. (receipt.reason or ""))
    end

    self.sent[key] = session.now()
    self.pending = nil
    self.retry_at = nil
    return true
end

function Party:request(key, command, arguments)
    if self.sent[key] then
        if session.now() - self.sent[key] > 30 then
            fail("party_confirmation_missing", "No server confirmation for " .. key .. "; the sent command was not repeated")
        end
        return
    end
    if not self:settle() then return end
    self.attempts[key] = (self.attempts[key] or 0) + 1
    self.pending = {key = key, id = command(arguments)}
end

function Party:clear_ready()
    local states = {}
    for index, value in pairs(self.room_states) do
        if value == 2 then states[index] = value end
    end
    self.room_states = states
end

function Party:observe(packet)
    if packet.direction ~= "ServerToClient" then return end
    local fields = packet.fields
    if packet.code == 94 and fields.subcommand == 15 then
        if packet.status ~= "decoded" and packet.status ~= "partial" then return end
        if type(fields.player_states) ~= "table" or #fields.player_states ~= 8 then return end
        self.room_states = fields.player_states
        self.readiness_tail_bytes = packet.remaining
        return
    end
    if packet.status ~= "decoded" then return end

    if packet.code == 94 then
        if fields.subcommand == 0 then
            self.room_states = {}
            self.mission_states = {}
            self.sent = {}
            self.attempts = {}
            self.room_id = fields.room_id
        elseif fields.subcommand == 2 then
            self.setup_revision = self.setup_revision + 1
            self:clear_ready()
        elseif fields.subcommand == 5 then
            self.mission_states[tostring(fields.player_id)] = nil
            self.room_states = {}
        end
    elseif packet.code == 91 then
        if fields.subcommand == 112 or fields.subcommand == 115 then
            self.mission_revision = self.mission_revision + 1
            self.mission_states = {}
            self:clear_ready()
        elseif fields.subcommand == 116 then
            self.mission_states[tostring(fields.player_id)] = fields.ready
        elseif fields.subcommand == 101 then
            self:clear_ready()
            self.mission_states = {}
        end
    end
end

function Party:member_state(current, id)
    for _, member in ipairs(current.players) do
        if member.id == id and member.place ~= nil then
            return self.room_states[member.place + 1]
        end
    end
end

function Party:prepare_follower(current)
    if not current.id then
        local invitations = room.invitations()
        for index = #invitations, 1, -1 do
            local invitation = invitations[index]
            if invitation.player_id == self.leader_id then
                local key = "join:" .. invitation.room_id
                self:request(key, room.join, {
                    login_flag = true, room_type = 4, room_id = -1,
                    argument = invitation.room_id, password = self.config.password,
                })
                return "Waiting for the leader's room entry confirmation"
            end
        end
        return "Waiting for the main character's invitation"
    end

    self:check_members(current)
    if current.map_id ~= self.config.map_id or current.difficulty ~= self.config.difficulty then
        return "Waiting for the leader's Formigueiro / Normal configuration"
    end
    if self:member_state(current, self.leader_id) ~= 2 then
        return "Waiting for server confirmation that the main character owns the room"
    end
    if self:member_state(current, self.own_id) == 1 then
        return "Waiting for the main character to start the ready party"
    end

    self:request("ready:" .. current.id .. ":" .. self.setup_revision, room.ready, {state = 1})
    return "Waiting for server room-ready confirmation after inventory collection"
end

function Party:prepare_leader(current)
    self:check_members(current)
    local present = {}
    for _, member in ipairs(current.players) do present[tostring(member.id)] = true end
    for _, id in ipairs(self.order) do
        if not present[tostring(id)] then
            self:request("invite:" .. current.id .. ":" .. id, room.invite, {player_id = id})
            return false, "Waiting for invited character " .. id .. " to join"
        end
    end
    if self:member_state(current, self.leader_id) ~= 2 then
        return false, "Waiting for server room-owner confirmation"
    end
    for _, id in ipairs(self.order) do
        if id ~= self.leader_id and self:member_state(current, id) ~= 1 then
            return false, "Waiting for character " .. id .. " to collect inventory and become ready"
        end
    end
    return true, "All four members are present and the three followers are ready"
end

function Party:prepare_mission(current)
    self:check_members(current)
    if not self:is_leader() then
        if self.mission_states[tostring(self.own_id)] ~= true then
            self:request("mission_ready:" .. self.mission_revision, fight.mission_ready, {ready = true})
            return false, "Waiting for server mission-ready confirmation after loot delivery"
        end
        if self:member_state(current, self.own_id) ~= 1 then
            self:request("mission_room_ready:" .. self.mission_revision, room.ready, {state = 1})
        end
        return false, "Loot delivered; waiting for the main character to advance the party"
    end

    if #current.players ~= #self.order then return false, "Waiting for all four room members" end
    for _, id in ipairs(self.order) do
        if id ~= self.leader_id and (self.mission_states[tostring(id)] ~= true or self:member_state(current, id) ~= 1) then
            return false, "Waiting for character " .. id .. " to confirm loot delivery and mission readiness"
        end
    end
    return true, "All followers confirmed mission readiness after their loot delivery"
end

function Party:card_indices(available)
    local choices = {}
    for _, index in ipairs(available) do
        if index % #self.order == self.ordinal then choices[#choices + 1] = index end
    end
    return choices
end

function Party:status()
    local current = room.state()
    local members = {}
    for _, id in ipairs(self.order) do
        members[#members + 1] = {
            player_id = id,
            room_state = self:member_state(current, id),
            mission_ready = self.mission_states[tostring(id)],
        }
    end
    return {
        leader_id = self.leader_id, members = members,
        pending = self.pending, setup_revision = self.setup_revision,
        mission_revision = self.mission_revision, readiness_tail_bytes = self.readiness_tail_bytes,
    }
end

return M
