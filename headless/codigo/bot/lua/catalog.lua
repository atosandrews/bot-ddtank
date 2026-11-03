local M = {}

local function fail(message)
    error({code = "catalog_unavailable", message = message}, 0)
end

local function copy(value)
    if type(value) ~= "table" then return value end
    local result = {}
    for key, entry in pairs(value) do result[key] = copy(entry) end
    return result
end

local function record(kind, args)
    if type(args) ~= "table" then fail("query requires an id table") end
    for key in pairs(args) do
        if key ~= "id" then fail("unknown query field: " .. tostring(key)) end
    end
    local id = args.id
    if type(id) ~= "number" or id ~= math.floor(id) or id < -2147483648 or id > 2147483647 then
        fail("id must be a signed 32-bit integer")
    end
    local data = require("data." .. kind)
    if not data[tostring(id)] then fail(kind .. " record absent: " .. tostring(id)) end
    return copy(data[tostring(id)])
end

function M.item(args) return record("items", args) end
function M.weapon(args) return record("weapons", args) end
function M.projectile(args) return record("projectiles", args) end
function M.map(args) return record("maps", args) end

return M
