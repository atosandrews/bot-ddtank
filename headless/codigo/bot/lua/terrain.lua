local catalog = require("catalog")
local M = {}
local maps = {}

local function fail(message)
    error({code = "terrain_unavailable", message = message}, 0)
end

local function coordinate(value)
    if type(value) ~= "number" or value ~= value or math.abs(value) > 1000000 then
        fail("coordinates must be finite and within one million pixels")
    end
    return value
end

local function fields(args, allowed)
    if type(args) ~= "table" then fail("query requires a table") end
    for key in pairs(args) do
        if not allowed[key] then fail("unknown query field: " .. tostring(key)) end
    end
end

local function ground(id)
    local info = catalog.map({id = id})
    if info.foreground ~= "" then fail("destructible terrain requires crater reconstruction") end
    if not info.terrain then fail("terrain record absent for map " .. tostring(id)) end
    if maps[id] then return maps[id] end
    local data = require(info.terrain)
    if data.width ~= info.width or data.height ~= info.height or type(data.rows) ~= "table" then
        fail("terrain dimensions or rows do not match map")
    end
    for y, row in pairs(data.rows) do
        if type(y) ~= "number" or y ~= math.floor(y) or y < 0 or y >= data.height or type(row) ~= "table" or #row % 2 ~= 0 then
            fail("invalid terrain row")
        end
        local previous = -1
        for index, x in ipairs(row) do
            if type(x) ~= "number" or x ~= math.floor(x) or x <= previous or x < 0 or x > data.width then
                fail("invalid terrain span")
            end
            previous = x
        end
    end
    maps[id] = data
    return data
end

local function solid(data, x, y)
    if x < 0 or y < 0 or x >= data.width or y >= data.height then return false end
    local row = data.rows[y]
    if not row then return false end
    local first, last = 1, #row / 2
    while first <= last do
        local middle = math.floor((first + last) / 2)
        local start, finish = row[2 * middle - 1], row[2 * middle]
        if x < start then last = middle - 1
        elseif x >= finish then first = middle + 1
        else return true end
    end
    return false
end

local function trunc(value)
    if value < 0 then return math.ceil(value) end
    return math.floor(value)
end

function M.solid(args)
    fields(args, {map_id = true, x = true, y = true})
    return solid(ground(args.map_id), trunc(coordinate(args.x)), trunc(coordinate(args.y)))
end

function M.trace(args)
    fields(args, {map_id = true, x = true, y = true, to_x = true, to_y = true})
    local data = ground(args.map_id)
    local x, y = coordinate(args.x), coordinate(args.y)
    local end_x, end_y = coordinate(args.to_x), coordinate(args.to_y)
    local dx, dy = end_x - x, end_y - y
    if math.abs(dx) + math.abs(dy) > 65536 then fail("segment exceeds 65536 pixels") end
    local cell_x, cell_y = math.floor(x), math.floor(y)
    local step_x, step_y = dx < 0 and -1 or 1, dy < 0 and -1 or 1
    local next_x, next_y = math.huge, math.huge
    if dx ~= 0 then next_x = (cell_x + (step_x > 0 and 1 or 0) - x) / dx end
    if dy ~= 0 then next_y = (cell_y + (step_y > 0 and 1 or 0) - y) / dy end
    local fraction = 0
    while fraction <= 1 do
        if solid(data, cell_x, cell_y) then
            return {hit = true, x = x + dx * fraction, y = y + dy * fraction, fraction = fraction}
        end
        if next_x == math.huge and next_y == math.huge then break end
        if next_x < next_y then
            fraction = next_x
            cell_x = cell_x + step_x
            next_x = (cell_x + (step_x > 0 and 1 or 0) - x) / dx
        elseif next_y < next_x then
            fraction = next_y
            cell_y = cell_y + step_y
            next_y = (cell_y + (step_y > 0 and 1 or 0) - y) / dy
        else
            fraction = next_x
            cell_x, cell_y = cell_x + step_x, cell_y + step_y
            next_x = (cell_x + (step_x > 0 and 1 or 0) - x) / dx
            next_y = (cell_y + (step_y > 0 and 1 or 0) - y) / dy
        end
    end
    return {hit = false, x = end_x, y = end_y, fraction = 1}
end

return M
