package packets

func playerInfo(r *reader, fields map[string]any) error {
	fields["gp"] = r.long("gp")
	for _, name := range []string{"offer", "riches_offer", "riches_rob", "win_count", "total_count", "escape_count", "attack", "defence", "agility", "luck", "damage", "armor", "hp", "hide"} {
		fields[name] = r.i32(name)
	}
	for _, name := range []string{"style", "colors", "skin"} {
		fields[name] = r.utf(name)
	}
	fields["show_consortia"] = r.boolean("show_consortia")
	fields["consortia_id"] = r.i32("consortia_id")
	fields["consortia_name"] = r.utf("consortia_name")
	for _, name := range []string{"badge_id", "unassigned_int_1", "unassigned_int_2", "nimbus"} {
		fields[name] = r.i32(name)
	}
	fields["pve_permission"] = r.utf("pve_permission")
	fields["fight_lib_mission"] = r.utf("fight_lib_mission")
	fields["fight_power"] = r.long("fight_power")
	fields["apprenticeship_state"] = r.i32("apprenticeship_state")
	fields["master_id"] = r.i32("master_id")
	fields["master_or_apprentices"] = r.utf("master_or_apprentices")
	fields["graduates_count"] = r.i32("graduates_count")
	fields["honour_of_master"] = r.utf("honour_of_master")
	fields["achievement_points"] = r.i32("achievement_points")
	fields["honor"] = r.utf("honor")
	fields["king_bless_open"] = r.boolean("king_bless_open")
	fields["free_boss_count"] = r.i32("free_boss_count")
	fields["last_spa_date"] = r.date("last_spa_date")
	fields["charm_gp"] = r.i32("charm_gp")
	fields["unassigned_int_3"] = r.i32("unassigned_int_3")
	fields["shop_finally_gotten_time"] = r.date("shop_finally_gotten_time")
	for _, name := range []string{"use_offer", "daily_score", "daily_wins", "daily_games", "weekly_score", "weekly_games", "weekly_ranking", "speed_training_exp", "attack_training_exp", "defence_training_exp", "hp_training_exp", "luck_training_exp", "training_task_count", "training_count"} {
		fields[name] = r.i32(name)
	}
	fields["training_task_date"] = r.date("training_task_date")
	count := r.count("dungeon_flag_count", 5)
	flags := make([]map[string]any, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		id := r.i32("dungeon_id")
		flag := r.i8("dungeon_flag")
		flags = append(flags, map[string]any{"id": id, "flag": flag})
	}
	fields["dungeon_flags"] = flags
	fields["evolution_grade"] = r.i32("evolution_grade")
	fields["evolution_exp"] = r.i32("evolution_exp")
	fields["dd_pass"] = r.boolean("dd_pass")
	return r.err
}
