# DDTank game data

These Lua tables are runtime data. Catalog keys are decimal strings to avoid sparse numeric-array allocation in the Lua VM; query functions accept numeric IDs. They do not load the launcher or read its cache. Deploy them with `catalog.lua`, `terrain.lua` and `trajectory.lua` using Bot YAML `lua_dir: lua`.

The records were converted on 2026-10-20 from the locally verified DD Clássico catalogs and terrain masks listed below. The original conversion used the Go catalog exporter, removed on 2026-10-22 with protocol-builder's Go application. Future extraction belongs in the [protocol research workspace](../../../../protocol-builder/README.md). Its source manifest is `protocol-builder/config/client-assets-20261020.json`; it records cache-entry names and decoded hashes for development reproduction. No account state, capture payloads or session keys are included.

| Development input | Decoded SHA-256 |
| --- | --- |
| `BallList.xml` | `978d439623b9def553fafb80885dc8b3a458ef7d26237bc0a562954a4cbcf1b8` |
| `LoadMapsItems.xml` | `13d716a25035c23e2e99924c642984e8aff5ce60603729939034040fe714dcab` |
| `BombConfig.xml` | `1e132a074f8c3a6c58768b094596afd90100f9bd59a465a309bd778f7316414e` |
| `TemplateAllList.xml` | `4f1a341d1b428113684154550de9d43b8e4a67a9a945431250a8e1dd644835ba` |
| `maps/1120/dead.png` | `0d04d76f42caaf1563e863f035dbcccc012794b568bf9f8bd9d3e765600a4dc6` |
| `maps/1121/dead.png` | `4b400df664f1a76e186dbdac35e3a08f21628fc5599a73e795053d8084758c89` |

`items.lua` imports the three group barrels and returns one shared item container. Each group barrel first imports individual item modules into one table, then builds its ID map in a second pass. This uses a fixed number of local variables regardless of item count. For example, `items/resources/pontos_de_pescaria_n2000.lua` returns the fishing-points record. Module names use the item name and ID so duplicate names remain distinct. Negative IDs use an `n` prefix in filenames only.

| Directory | Contents | Source category IDs |
| --- | --- | --- |
| `items/skins/` | Hair, facial accessories, faces, outfits, wings and chat bubbles | 3, 4, 6, 13, 15, 16 |
| `items/equipments/` | Headgear, eyewear, armor, weapons, jewelry, offhands, healing stones, equipment cards, badges, pet gear and socket equipment | 1, 2, 5, 7, 8, 9, 14, 17, 19, 26, 27, 28, 29, 31, 40, 50, 51, 52, 61, 64, 68, 69, 70, 74 |
| `items/resources/` | Currencies, consumables, crafting and upgrade materials, quest items, boxes and other utility records | 10, 11, 12, 18, 20, 21, 22, 23, 24, 25, 30, 32, 33, 34, 35, 36, 37, 43, 53, 60, 62, 63, 65, 66, 71, 72, 73, 76, 78, 80, 81, 900 |

This is the project's grouping policy, using the client's `EquipType` definitions and template categories. Skins can still carry statistics. The original `category_id` and all exported statistics remain unchanged; `CanEquip` alone does not determine the group because the source also marks some fragments as equippable. Unknown categories stop export for review instead of silently assigning a group. The current catalog has 6,252 resources, 4,210 equipment records and 2,614 skins.

Each record contains the selected item schema: ID, name, category, available base statistics, level requirements, quality and raw property strings. `weapons.lua` contains numeric projectile selections and angle limits after BombConfig overrides. `projectiles.lua` and `maps.lua` contain the verified physics coefficients.

`terrain/1120.lua` and `terrain/1121.lua` contain exact solid pixel spans for the two Formigueiro stages. A row keyed by zero-based y contains alternating start and exclusive-end x coordinates. Missing rows are empty. Solid means source alpha greater than 150. No image files are needed at runtime. Other maps have no terrain module unless one has been explicitly exported and added to their record.

Catalog records are static definitions. Live equipment, position, wind, HP and fight state still come from protocol events. The room instance selection ID is not necessarily the active stage map ID; select geometry using `fight.state().map_id`.

See [PHYSICS-API.md](../../docs/PHYSICS-API.md) for queries, regeneration and limits. Keep the provenance hashes when updating records; do not substitute guessed physics or empty terrain for missing data.
