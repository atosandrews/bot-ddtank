"""Gera instances/pintinho.lua a partir de formigueiro.lua (que não é alterado)."""
import sys
from pathlib import Path
base = Path(sys.argv[1])
src = (base / "formigueiro.lua").read_text(encoding="utf-8")
def troca(velho, novo, n=1):
    global src
    c = src.count(velho)
    assert c == n, (c, velho)
    src = src.replace(velho, novo)
troca('''local M = {}
''', '''-- Pintinho Corre Rápido (instância 7, Difícil = 2), derivado do motor do Formigueiro:
-- mira pelo terreno exato de cada estágio, adereços, cartas, captcha e coleta da mochila.
-- Estágios conhecidos ficam em config.stage_maps; um mapa novo pausa a partida (sem chute).
local M = {}
''')
troca('''        room_name = "Formigueiro solo",
        password = "solo",
        map_id = 2,
        difficulty = 1,''', '''        room_name = "Pintinho",
        password = "pinto",
        map_id = 7,
        difficulty = 2,
        -- Mapas dos estágios com terreno exportado (data/terrain/<id>.lua).
        stage_maps = {1161, 1163},
        -- Estágios com alvos agrupados: o tridente (3 projéteis) entra no lugar do ataque extra.
        trident_maps = {["1161"] = true},
        -- Alvos por nome: os primeiros da lista antes; "last" só quando não sobrar outro.
        target_first = {"Burrinho", "Galinha"},
        target_last = {"Gaiola"},''')
troca('''            assert(config[key] ~= nil, "Unknown Formigueiro option: " .. key)''',
      '''            assert(config[key] ~= nil, "Unknown Pintinho option: " .. key)''')
troca('''    for _, map_id in ipairs({1120, 1121}) do
        terrain.solid({map_id = map_id, x = 0, y = 0})
    end''', '''    local supported_maps = {}
    for _, map_id in ipairs(config.stage_maps) do
        terrain.solid({map_id = map_id, x = 0, y = 0})
        supported_maps[tostring(map_id)] = true
    end

    local function name_rank(name, list)
        name = string.lower(tostring(name or ""))
        for index, pattern in ipairs(list) do
            if string.find(name, string.lower(pattern), 1, true) then return index end
        end
        return nil
    end''')
troca('''                candidates[#candidates + 1] = {
                    id = actor.id, max_hp = actor.max_hp, position = {x = position.x, y = position.y},
                    position_source = source,
                }''', '''                candidates[#candidates + 1] = {
                    id = actor.id, max_hp = actor.max_hp, position = {x = position.x, y = position.y},
                    position_source = source, name = actor.name,
                    first = name_rank(actor.name, config.target_first) or 1000,
                    last = name_rank(actor.name, config.target_last) and 1 or 0,
                }''')
troca('''            if fight_state.map_id == 1121 and (a.max_hp or 0) ~= (b.max_hp or 0) then
                return (a.max_hp or 0) > (b.max_hp or 0)
            end
''', '''            if a.last ~= b.last then return a.last < b.last end
            if a.first ~= b.first then return a.first < b.first end
''')
troca('''        if fight_state.map_id == 1120 then primary = trident end''',
      '''        if config.trident_maps[tostring(fight_state.map_id)] then primary = trident end''')
troca('''                if fight_state.map_id == 1120 then return plan end''',
      '''                if config.trident_maps[tostring(fight_state.map_id)] then return plan end''')
troca('''            elseif fight_state.map_id == 1120 and not state.trident_allowed then''',
      '''            elseif config.trident_maps[tostring(fight_state.map_id)] and not state.trident_allowed then''')
troca('''        if fight_state.map_id ~= 1120 or self.special then return false end''',
      '''        if not config.trident_maps[tostring(fight_state.map_id)] or self.special then return false end''')
troca('''            if fight_state.map_id ~= 1120 and fight_state.map_id ~= 1121 then''',
      '''            if not supported_maps[tostring(fight_state.map_id)] then''')
troca('"Waiting for Formigueiro / Normal confirmation"', '"Waiting for Pintinho / Difícil confirmation"')
troca('"Server room configuration does not match Formigueiro / Normal"', '"Server room configuration does not match Pintinho / Difícil"')
troca('function formigueiro_stop_after_run()', 'function pintinho_stop_after_run()')
troca('function formigueiro_status()', 'function pintinho_status()')
troca('inspect formigueiro_status()', 'inspect pintinho_status()')
assert "1120" not in src and "1121" not in src, "restou mapa do Formigueiro"
(base / "pintinho.lua").write_text(src, encoding="utf-8")
print("ok", len(src.splitlines()))
