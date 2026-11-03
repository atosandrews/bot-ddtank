local resources = require("data.items.resources")
local equipments = require("data.items.equipments")
local skins = require("data.items.skins")
local items = {}

for _, group in ipairs({resources, equipments, skins}) do
    for _, item in pairs(group) do
        local id = tostring(item.id)
        if items[id] then
            error({code = "catalog_duplicate", message = "duplicate item ID: " .. id}, 0)
        end
        items[id] = item
    end
end

return items
