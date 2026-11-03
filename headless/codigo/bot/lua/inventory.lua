local M = {}
local Organizer = {}
Organizer.__index = Organizer

local function fail(code, message)
    error({code = code, message = message}, 0)
end

local function snapshot(bag)
    local state = player.state()
    if state.inventory_uncertain or not state.bag_revisions then
        fail("inventory_unknown", "Organization requires reliable bag revisions from the updated Bot")
    end

    local totals, items = {}, {}
    for _, item in ipairs(player.inventory()) do
        if item.bag_type == bag and (bag == 1 or item.place >= 31) then
            items[#items + 1] = item
            local key = tostring(item.template_id)
            totals[key] = (totals[key] or 0) + item.count
        end
    end

    return {
        totals = totals, items = items,
        revision = state.inventory_revision,
        bag_revision = state.bag_revisions[tostring(bag)] or 0,
    }
end

local function same_totals(first, second)
    for template, count in pairs(first) do
        if second[template] ~= count then return false end
    end
    for template, count in pairs(second) do
        if first[template] ~= count then return false end
    end
    return true
end

local function duplicated(items)
    local seen = {}
    for _, item in ipairs(items) do
        local key = tostring(item.template_id)
        if seen[key] then return true end
        seen[key] = true
    end
    return false
end

function M.new()
    return setmetatable({bag = 0, completed = {}, attempts = 0}, Organizer)
end

function Organizer:step()
    if self.bag > 1 then return true, "Inventory organization confirmed" end

    local current = snapshot(self.bag)
    if self.action then
        local receipt = actions.get(self.action)
        if not receipt then fail("inventory_receipt_missing", "Organization receipt is unavailable") end
        if receipt.status == "rejected" and receipt.reason == "stale_state" then
            self.action = nil
            self.retry_at = session.now() + math.min(2, 0.1 * 2 ^ math.min(self.attempts - 1, 5))
            return false, "Organization was not written; waiting to use fresh inventory"
        end
        if receipt.status == "rejected" or receipt.status == "cancelled" or receipt.status == "uncertain" then
            fail("inventory_organization_failed", "Organization " .. receipt.status .. ": " .. (receipt.reason or ""))
        end

        if receipt.status == "sent" and current.bag_revision > self.baseline.bag_revision then
            if not same_totals(self.baseline.totals, current.totals) then
                fail("inventory_quantity_changed", "Item totals changed during organization; reconciliation required")
            end
            if #current.items > #self.baseline.items then
                fail("inventory_slots_increased", "Organization increased occupied slots; reconciliation required")
            end
            self.completed[tostring(self.bag)] = {
                before = #self.baseline.items, after = #current.items,
                revision = current.bag_revision, quantities_preserved = true,
            }
            self.bag = self.bag + 1
            self.action = nil
            self.attempts = 0
            self.retry_at = nil
            return false, "Bag organization confirmed with unchanged item quantities"
        end

        -- Only a command the relay actually wrote can leave the server state uncertain.
        if receipt.status == "sent" and session.now() - self.started >= 60 then
            fail("inventory_organization_unconfirmed", "No confirmed bag update after organization; request was not repeated")
        end
        return false, "Waiting for the server to organize inventory"
    end

    if not duplicated(current.items) then
        self.bag = self.bag + 1
        return false, "Bag has no duplicate item types to consolidate"
    end
    if self.retry_at and session.now() < self.retry_at then return false, "Waiting to organize fresh inventory" end

    table.sort(current.items, function(a, b)
        if a.template_id ~= b.template_id then return a.template_id < b.template_id end
        return a.place < b.place
    end)
    local places = {}
    for _, item in ipairs(current.items) do places[#places + 1] = item.place end

    self.baseline = current
    self.action = player.organize_inventory({bag_type = self.bag, places = places, revision = current.revision})
    self.started = session.now()
    self.attempts = self.attempts + 1
    return false, "Organizing inventory before reward collection"
end

function Organizer:status()
    return {bag = self.bag, action = self.action, attempts = self.attempts, completed = self.completed}
end

return M
