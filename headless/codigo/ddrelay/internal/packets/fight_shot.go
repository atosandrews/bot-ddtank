package packets

var bombActionNames = map[int32]string{
	1: "PICK", 2: "BOMB", 3: "START_MOVE", 4: "FLY_OUT", 5: "KILL_PLAYER",
	6: "TRANSLATE", 7: "FORZEN", 8: "CHANGE_SPEED", 9: "UNFORZEN", 10: "DANER",
	11: "CURE", 12: "GEM_DEFENSE_CHANGED", 13: "CHANGE_STATE", 14: "DO_ACTION",
	15: "PLAYBUFFER", 16: "Laser", 17: "BOMBED", 18: "PUP", 19: "AUP", 20: "PET",
}

var bombActionParameters = map[int32][4]string{
	1: {"object_id"}, 2: {"x", "y"}, 3: {"target_id", "x", "y", "movement_flag"},
	5: {"target_id", "damage", "hp_effect", "hp"}, 6: {"x", "y"}, 7: {"target_id"},
	8: {"vx", "vy", "effect_target_id"}, 9: {"target_id"}, 10: {"target_id", "dander"},
	11: {"target_id", "hp", "healing"}, 12: {"target_id"}, 13: {"target_id", "state"},
	14: {"target_id", "", "", "action_index"}, 17: {"target_id"}, 18: {"target_id"},
	19: {"target_id"}, 20: {"target_id"},
}

func fightShot(r *reader, f map[string]any) error {
	fightWind(r, f)
	f["projectiles"] = fightEntries(r, "projectiles", 47, func(e map[string]any) {
		fightInts(r, e, "number", "shoot_count")
		e["creates_hole"] = r.boolean("creates_hole")
		fightInts(r, e, "id", "x", "y", "vx", "vy", "template_id")
		e["changed_particle"] = r.utf("changed_particle")
		fightInts(r, e, "damage_modifier_1_thousandths", "damage_modifier_2_thousandths")
		e["actions"] = fightEntries(r, "actions", 24, func(a map[string]any) {
			fightInts(r, a, "time", "type", "parameter_1", "parameter_2", "parameter_3", "parameter_4")
			name, ok := bombActionNames[a["type"].(int32)]
			if !ok {
				name = "UNKNOWN"
			}
			a["name"] = name
			keys := [4]string{"parameter_1", "parameter_2", "parameter_3", "parameter_4"}
			for i, field := range bombActionParameters[a["type"].(int32)] {
				if field != "" {
					a[field] = a[keys[i]]
				}
			}
		})
	})
	f["pet_targets"] = fightEntries(r, "pet_targets", 16, func(e map[string]any) {
		fightInts(r, e, "target_id", "damage", "hp", "dander")
	})
	f["pet_attack_type"] = r.i32("pet_attack_type")
	return r.err
}
