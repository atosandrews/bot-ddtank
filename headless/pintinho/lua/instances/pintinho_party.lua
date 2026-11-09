-- Grupo do Pintinho (separado do party.lua do Formigueiro, que não muda):
-- a principal (conta forte) faz os estágios antes do último sozinha; antes do último estágio
-- convida as convidadas, espera todas entrarem e darem Pronto, e só então avança. No fim da
-- partida as convidadas saem da sala sozinhas e a principal recomeça sozinha.
-- Mesma interface que o motor usa do party.lua: settle, is_leader, check_members, observe,
-- prepare_follower, prepare_leader, prepare_mission, card_indices, status.
local M = {}
local Party = {}
Party.__index = Party

local INVITE_AGAIN = 15   -- segundos entre convites repetidos à mesma convidada
local CONFIRM_TIMEOUT = 30

local function fail(code, message)
    error({code = code, message = message}, 0)
end

-- options = {leader_id = <forte>, guests = {id1, id2, id3}, invite_after_stages = 3}
function M.new(options, config)
    local own_id = player.state().id
    local allowed = {[tostring(options.leader_id)] = true}
    local ordinal = own_id == options.leader_id and 0 or nil
    for index, id in ipairs(options.guests) do
        if allowed[tostring(id)] then fail("party_config", "Repeated party member") end
        allowed[tostring(id)] = true
        if id == own_id then ordinal = index end
    end
    if not ordinal then fail("party_config", "This character is not in the Pintinho party") end

    return setmetatable({
        own_id = own_id,
        leader_id = options.leader_id,
        guests = options.guests,
        allowed = allowed,
        ordinal = ordinal,
        invite_after = options.invite_after_stages or 3,
        config = config,
        room_states = {},
        sent = {},
        attempts = {},
        setup_revision = 0,
        invited_at = {},
    }, Party)
end

function Party:is_leader()
    return self.own_id == self.leader_id
end

function Party:check_members(current)
    for _, member in ipairs(current.players) do
        if not self.allowed[tostring(member.id)] then
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
    if receipt.status == "rejected" and receipt.reason == "stale_state" and (self.attempts[key] or 0) < 5 then
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

function Party:request(key, command, arguments, repeatable)
    if self.sent[key] then
        if not repeatable and session.now() - self.sent[key] > CONFIRM_TIMEOUT then
            fail("party_confirmation_missing", "No server confirmation for " .. key .. "; the sent command was not repeated")
        end
        return
    end
    if not self:settle() then return end
    self.attempts[key] = (self.attempts[key] or 0) + 1
    self.pending = {key = key, id = command(arguments)}
end

function Party:observe(packet)
    if packet.direction ~= "ServerToClient" then return end
    local fields = packet.fields
    if packet.code == 94 and fields.subcommand == 15 then
        if packet.status ~= "decoded" and packet.status ~= "partial" then return end
        if type(fields.player_states) ~= "table" or #fields.player_states ~= 8 then return end
        self.room_states = fields.player_states
        return
    end
    if packet.status ~= "decoded" then return end
    if packet.code == 94 then
        if fields.subcommand == 0 then
            self.room_states = {}
            self.sent = {}
            self.attempts = {}
            self.invited_at = {}
        elseif fields.subcommand == 2 then
            self.setup_revision = self.setup_revision + 1
        elseif fields.subcommand == 5 then
            self.room_states = {}
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

-- Convidada: entra pelo convite da principal e dá Pronto. A saída depois da partida é pedida
-- pelo motor (leave_after_run).
function Party:prepare_follower(current)
    -- Depois da partida: sai da sala, repetindo a cada 3 s até o servidor confirmar (logo após a
    -- luta o servidor ainda está na tela de resultado e ignora a saída).
    if self.leave_wanted then
        if not current.id then
            -- logo após a luta a sala some por instantes e volta: só vale depois de 5 s sem sala
            self.gone_since = self.gone_since or session.now()
            if session.now() - self.gone_since < 5 then return "Confirming that the room was left" end
            self.leave_wanted = nil
            self.gone_since = nil
        else
            self.gone_since = nil
            if not self.last_leave or session.now() - self.last_leave >= 3 then
                if not self:settle() then return "Waiting to leave the room" end
                self.last_leave = session.now()
                self.attempts.leave = (self.attempts.leave or 0) + 1
                self.pending = {key = "leave", id = room.leave()}
                self.sent.leave = nil
            end
            return "Leaving the room after the last stage"
        end
    end
    if not current.id then
        local invitations = room.invitations()
        for index = #invitations, 1, -1 do
            local invitation = invitations[index]
            if invitation.player_id == self.leader_id then
                self:request("join:" .. invitation.room_id .. ":" .. tostring(invitation.revision or index), room.join, {
                    login_flag = true, room_type = 4, room_id = -1,
                    argument = invitation.room_id, password = self.config.password,
                }, true)
                return "Waiting for the leader's room entry confirmation"
            end
        end
        return "Waiting for the main character's invitation for the last stage"
    end

    self:check_members(current)
    if current.map_id ~= self.config.map_id or current.difficulty ~= self.config.difficulty then
        return "Waiting for the leader's Pintinho / Difícil configuration"
    end
    if self:member_state(current, self.own_id) == 1 then
        return "Ready; waiting for the main character to start the last stage"
    end
    self:request("ready:" .. current.id .. ":" .. self.setup_revision, room.ready, {state = 1})
    return "Waiting for server room-ready confirmation"
end

-- Início de cada partida: a principal joga sozinha; espera as convidadas da partida anterior saírem.
function Party:prepare_leader(current)
    self:check_members(current)
    if #current.players > 1 then
        return false, "Waiting for the guests of the previous run to leave the room"
    end
    if self:member_state(current, self.leader_id) ~= nil and self:member_state(current, self.leader_id) ~= 2 then
        return false, "Waiting for server room-owner confirmation"
    end
    return true, "Main character alone for the solo stages"
end

-- Antes de cada estágio seguinte. stages_done = estágios já vencidos nesta partida.
function Party:prepare_mission(current, stages_done)
    self:check_members(current)
    if not self:is_leader() then
        return false, "Waiting for the main character to advance"
    end
    if (stages_done or 0) < self.invite_after then
        return true, "Solo stage"
    end

    local present = {}
    for _, member in ipairs(current.players) do present[tostring(member.id)] = true end
    for _, id in ipairs(self.guests) do
        if not present[tostring(id)] then
            local last = self.invited_at[tostring(id)]
            if not last or session.now() - last >= INVITE_AGAIN then
                if not self:settle() then return false, "Waiting to send the invitation" end
                self.invited_at[tostring(id)] = session.now()
                local key = "invite:" .. id
                self.attempts[key] = (self.attempts[key] or 0) + 1
                self.sent[key] = nil
                self.pending = {key = key, id = room.invite({player_id = id})}
            end
            return false, "Waiting for guest " .. id .. " to join for the last stage"
        end
    end
    for _, id in ipairs(self.guests) do
        if self:member_state(current, id) ~= 1 then
            return false, "Waiting for guest " .. id .. " to become ready"
        end
    end
    return true, "All guests are present and ready for the last stage"
end

-- Cartas: sozinha, qualquer carta; com as convidadas, cada uma na sua fatia (índice % 4).
function Party:card_indices(available)
    local players = 0
    for _, actor in ipairs(fight.state().actors or {}) do
        if actor.player_id then players = players + 1 end
    end
    if players <= 1 then return available end
    local total = #self.guests + 1
    local choices = {}
    for _, index in ipairs(available) do
        if index % total == self.ordinal then choices[#choices + 1] = index end
    end
    return choices
end

function Party:status()
    local current = room.state()
    local members = {{player_id = self.leader_id, room_state = self:member_state(current, self.leader_id)}}
    for _, id in ipairs(self.guests) do
        members[#members + 1] = {player_id = id, room_state = self:member_state(current, id)}
    end
    return {leader_id = self.leader_id, members = members, pending = self.pending,
        setup_revision = self.setup_revision, invite_after = self.invite_after}
end

return M
