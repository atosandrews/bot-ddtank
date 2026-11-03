local M = {}
local inventory_organization = require("inventory")
local Collector = {}
-- Seconds without the temporary bag emptying before treating the rewards as not fitting.
local NOT_FITTED_SECONDS = 6
-- Seconds after collecting, with the temporary bag empty, before treating missing items as mailed.
local MAIL_GRACE_SECONDS = 5
Collector.__index = Collector

local function fail(code, message)
    error({code = code, message = message}, 0)
end

local function inventory()
    local current = player.state()
    if current.inventory_uncertain or not current.inventory_revision then
        fail("reward_inventory_unknown", "Reward collection requires the updated Bot and reliable inventory state")
    end

    local permanent, temporary = {}, {}
    for _, item in ipairs(player.inventory()) do
        local totals
        if item.bag_type == 0 or item.bag_type == 1 then
            totals = permanent
        elseif item.bag_type == 4 then
            totals = temporary
        end

        if totals and item.template_id > 0 and item.count > 0 then
            local key = tostring(item.template_id)
            totals[key] = (totals[key] or 0) + item.count
        end
    end

    return permanent, temporary
end

function M.new()
    local permanent, temporary = inventory()
    local expected = {}
    for template, count in pairs(temporary) do expected[template] = count end

    return setmetatable({
        baseline = permanent,
        temporary = temporary,
        expected = expected,
        complete = false,
        attempts = 0,
        organization = inventory_organization.new(),
    }, Collector)
end

function Collector:observe()
    if self.complete then return end

    local _, temporary = inventory()
    for template, count in pairs(temporary) do
        local added = count - (self.temporary[template] or 0)
        if added > 0 then self.expected[template] = (self.expected[template] or 0) + added end
    end

    self.temporary = temporary
end

function Collector:step(awards)
    if self.complete then return true, "Permanent inventory delivery confirmed" end

    self:observe()

    -- Organizing costs a server round trip per bag and the party waits for the slowest member,
    -- so it only runs after a reward did not fit (the game's "Arrumar a mochila" + ok).
    if M.organize_next and not self.organization_skipped then
        -- Organizing only consolidates stacks; when the server does not confirm it, collect anyway.
        local ok, organized, reason = pcall(self.organization.step, self.organization)
        if not ok then
            self.organization_skipped = type(organized) == "table" and organized.message or tostring(organized)
            M.organize_next = false
        elseif not organized then
            return false, reason
        else
            M.organize_next = false
        end
    end

    self.started = self.started or session.now()

    for template, count in pairs(awards) do
        template = tostring(template)
        self.expected[template] = math.max(self.expected[template] or 0, count)
    end

    local permanent, temporary = inventory()
    local missing = false
    local available = true
    for template, count in pairs(self.expected) do
        local received = (permanent[template] or 0) - (self.baseline[template] or 0)
        local remaining = count - received
        if remaining > 0 then
            missing = true
            if (temporary[template] or 0) < remaining then available = false end
        end
    end

    if not missing and next(temporary) == nil then
        self.complete = true
        self.received = {}
        for template in pairs(self.expected) do
            self.received[template] = (permanent[template] or 0) - (self.baseline[template] or 0)
        end
        self.inventory_revision = player.state().inventory_revision
        return true, "Permanent inventory delivery confirmed"
    end

    -- With a full bag the server sends what does not fit to the mailbox: the temporary bag empties
    -- without the permanent bag gaining. Nothing is lost, so the run goes on at once.
    if missing and next(temporary) == nil and self.action and session.now() - self.started >= MAIL_GRACE_SECONDS then
        return self:mailed(permanent)
    end

    if self.action then
        local receipt = actions.get(self.action)
        if not receipt then fail("reward_receipt_missing", "The collection receipt is unavailable") end

        if receipt.status == "rejected" and receipt.reason == "stale_state" then
            self.action = nil
            self.retry_at = session.now() + math.min(2, 0.1 * 2 ^ math.min(self.attempts - 1, 5))
            self.started = self.retry_at
            return false, "Collection was not written; rechecking the latest inventory before retry"
        end

        if receipt.status == "rejected" or receipt.status == "cancelled" then
            fail("reward_request_rejected", "Collection was not sent: " .. (receipt.reason or receipt.status))
        end

        if receipt.status == "uncertain" then
            fail("reward_request_uncertain", "Collection outcome is uncertain; the request was not repeated")
        end

        if session.now() - self.started >= NOT_FITTED_SECONDS then
            return self:not_fitted(permanent)
        end

        return false, "Waiting for permanent inventory gains and temporary inventory removal"
    end

    if session.now() - self.started >= NOT_FITTED_SECONDS then
        return self:not_fitted(permanent)
    end

    if not available then return false, "Waiting for the server inventory to include all awarded items" end
    if self.retry_at and session.now() < self.retry_at then return false, "Waiting to recheck an unwritten collection request" end

    self.attempts = self.attempts + 1
    self.action = player.collect_rewards()
    self.started = session.now()
    return false, "Collecting temporary rewards"
end

-- Rewards still in the temporary bag after a collection: organize the bag right away (the game's
-- "Arrumar a mochila" + ok) and collect again; only a second failure closes the run without them.
function Collector:not_fitted(permanent)
    if self.organized_for_space then return self:give_up(permanent) end
    self.organized_for_space = true
    M.organize_next = true
    self.organization = inventory_organization.new()
    self.organization_skipped = nil
    self.action = nil
    self.started = nil
    return false, "Temporary rewards did not fit; organizing the bag before collecting again"
end

-- Items went to the mailbox: close the run and organize the bag at the next collection, so stacks
-- merge and later rewards (and the mailbox attachments, collected by hand) have room.
function Collector:mailed(permanent)
    self.complete = true
    self.sent_to_mail = true
    self.action = nil
    M.organize_next = true
    self.received = {}
    for template in pairs(self.expected) do
        self.received[template] = (permanent[template] or 0) - (self.baseline[template] or 0)
    end
    self.inventory_revision = player.state().inventory_revision
    return true, "Bag full: the server sent the remaining rewards to the mailbox"
end

-- A full permanent bag keeps rewards in the temporary bag. The run is closed with what arrived
-- (marked undelivered) so the farm keeps going instead of blocking the next mission.
function Collector:give_up(permanent)
    self.complete = true
    self.undelivered = true
    self.action = nil
    self.received = {}
    for template in pairs(self.expected) do
        self.received[template] = (permanent[template] or 0) - (self.baseline[template] or 0)
    end
    self.inventory_revision = player.state().inventory_revision
    return true, "Rewards did not fit in the permanent bag; continuing without them"
end

function Collector:status()
    local expected = {}
    for template, count in pairs(self.expected) do expected[tostring(template)] = count end

    local received = {}
    for template, count in pairs(self.received or {}) do received[tostring(template)] = count end
    return {
        complete = self.complete, action = self.action, expected = expected,
        received = received, inventory_revision = self.inventory_revision,
        attempts = self.attempts, retry_at = self.retry_at, undelivered = self.undelivered, sent_to_mail = self.sent_to_mail,
        organization = self.organization:status(),
    }
end

return M
