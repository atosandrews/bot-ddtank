package packets

func inventory(r *reader, f map[string]any) error {
	f["bag_type"] = r.i32("bag_type")
	f["slots"] = fightEntries(r, "slots", 5, func(e map[string]any) {
		e["place"] = r.i32("place")
		present := r.boolean("present")
		e["present"] = present
		if present {
			item := make(map[string]any)
			inventoryItem(r, item, false)
			e["item"] = item
		}
	})
	return r.err
}

func inventoryItem(r *reader, f map[string]any, equipment bool) {
	if equipment {
		f["bag_type"] = r.i8("bag_type")
	}
	fightInts(r, f, "user_id", "item_id", "count", "place", "template_id", "attack_compose", "defend_compose", "agility_compose", "luck_compose", "strengthen_level")
	if !equipment {
		f["strengthen_exp"] = r.i32("strengthen_exp")
	}
	fightFlags(r, f, "bound", "judged")
	f["begin_date"] = r.date("begin_date")
	f["valid_date"] = r.i32("valid_date")
	fightStrings(r, f, "color", "skin")
	f["used"] = r.boolean("used")
	fightInts(r, f, "hole_1", "hole_2", "hole_3", "hole_4", "hole_5", "hole_6")
	f["picture"] = r.utf("picture")
	f["refinery_level"] = r.i32("refinery_level")
	f["discolor_valid_date"] = r.date("discolor_valid_date")
	if !equipment {
		f["strengthen_times"] = r.i32("strengthen_times")
	}
	f["hole_5_level"] = r.i8("hole_5_level")
	f["hole_5_exp"] = r.i32("hole_5_exp")
	f["hole_6_level"] = r.i8("hole_6_level")
	f["hole_6_exp"] = r.i32("hole_6_exp")
	gold := r.boolean("gold")
	f["gold"] = gold
	if gold {
		f["gold_valid_date"] = r.i32("gold_valid_date")
		f["gold_begin_time"] = r.date("gold_begin_time")
	}
	fightStrings(r, f, "latent_energy_current", "latent_energy_new")
	f["latent_energy_end_time"] = r.date("latent_energy_end_time")
}

func equipment(r *reader, f map[string]any) error {
	f["player_id"] = r.i32("player_id")
	f["nickname"] = r.utf("nickname")
	fightInts(r, f, "agility", "attack", "damage", "armor")
	fightStrings(r, f, "colors", "skin")
	f["defence"] = r.i32("defence")
	f["gp"] = r.long("gp")
	fightInts(r, f, "grade", "luck", "hp", "hide", "repute")
	f["sex"] = r.boolean("sex")
	f["style"] = r.utf("style")
	f["offer"] = r.i32("offer")
	f["vip_type"] = r.i8("vip_type")
	f["vip_level"] = r.i32("vip_level")
	f["king_bless_open"] = r.boolean("king_bless_open")
	fightInts(r, f, "win_count", "total_count", "escape_count", "consortia_id")
	f["consortia_name"] = r.utf("consortia_name")
	fightInts(r, f, "badge_id", "riches_offer", "riches_rob")
	f["married"] = r.boolean("married")
	f["spouse_id"] = r.i32("spouse_id")
	fightStrings(r, f, "spouse_name", "duty_name")
	f["nimbus"] = r.i32("nimbus")
	f["fight_power"] = r.long("fight_power")
	apprenticeship(r, f)
	f["achievement_point"] = r.i32("achievement_point")
	f["honor"] = r.utf("honor")
	f["last_login_date"] = r.date("last_login_date")
	training(r, f)
	f["daily_league_first"] = r.boolean("daily_league_first")
	fightInts(r, f, "daily_league_last_score", "totem_id")
	f["items"] = fightEntries(r, "items", 118, func(i map[string]any) { inventoryItem(r, i, true) })
	return r.err
}
