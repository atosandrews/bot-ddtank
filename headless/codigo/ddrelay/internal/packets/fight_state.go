package packets

func fightTurn(r *reader, f map[string]any) error {
	fightWindDisplay(r, f)
	f["hidden"] = r.boolean("hidden")
	f["turn_time"] = r.i32("turn_time")
	f["boxes"] = fightEntries(r, "boxes", 16, func(e map[string]any) {
		fightInts(r, e, "id", "x", "y", "type")
	})
	if r.err == nil && r.offset == len(r.data) {
		f["layout"] = "common_prefix"
		return ErrUnsupported
	}
	f["layout"] = "player"
	f["livings"] = fightEntries(r, "livings", 50, func(e map[string]any) {
		e["living_id"] = r.i32("living_id")
		e["alive"] = r.boolean("alive")
		fightInts(r, e, "x", "y", "power_ratio", "max_hp", "hp")
		e["no_hole"] = r.boolean("no_hole")
		fightInts(r, e, "max_energy", "dander", "pet_max_mp", "pet_mp", "shoot_count", "fly_count")
	})
	f["turn_number"] = r.i32("turn_number")
	return r.err
}

func fightLiving(r *reader, f map[string]any) error {
	f["living_type"] = r.i8("living_type")
	f["living_id"] = r.i32("living_id")
	fightStrings(r, f, "name", "model", "action")
	fightInts(r, f, "x", "y", "hp", "max_hp", "team")
	f["direction"] = r.i8("direction")
	f["layer"] = r.i8("layer")
	fightFlags(r, f, "spawn_flag_1", "spawn_flag_2")
	f["action_mapping"] = fightProperties(r)
	f["buff_ids"] = fightIDs(r, "buff_ids")
	fightFlags(r, f, "frozen", "hidden", "no_hole", "state_flag")
	f["properties"] = fightProperties(r)
	return r.err
}

func fightStart(r *reader, f map[string]any) error {
	f["players"] = fightEntries(r, "players", 56, func(e map[string]any) {
		fightInts(r, e, "living_id", "x", "y", "direction", "hp", "max_hp", "team", "refinery_level", "power_ratio", "dander")
		e["buffs"] = fightEntries(r, "buffs", 8, func(b map[string]any) { fightInts(r, b, "id", "data") })
		e["out_turn_buff_ids"] = fightIDs(r, "out_turn_buff_ids")
		fightFlags(r, e, "frozen", "hidden", "no_hole", "state_flag")
		e["properties"] = fightProperties(r)
	})
	f["server_time"] = r.date("server_time")
	return r.err
}

func fightStyle(r *reader, f map[string]any) error {
	kind := r.i8("kind")
	f["kind"] = kind
	switch kind {
	case 0:
		f["players"] = fightEntries(r, "players", 17, func(e map[string]any) {
			e["style"] = r.utf("style")
			e["hide"] = r.i32("hide")
			e["sex"] = r.boolean("sex")
			fightStrings(r, e, "skin", "colors")
			e["player_id"] = r.i32("player_id")
			e["pet_style"] = r.utf("pet_style")
		})
	case 1:
		f["player_id"] = r.i32("player_id")
		f["use_pet_style"] = r.boolean("use_pet_style")
	default:
		return ErrUnsupported
	}
	return r.err
}
