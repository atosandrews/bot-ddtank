package packets

func apprenticeship(r *reader, f map[string]any) {
	fightInts(r, f, "apprenticeship_state", "master_id")
	f["master_or_apprentices"] = r.utf("master_or_apprentices")
	f["graduates_count"] = r.i32("graduates_count")
	f["master_honor"] = r.utf("master_honor")
}

func training(r *reader, f map[string]any) {
	fightInts(r, f, "speed_training_exp", "attack_training_exp", "defence_training_exp", "hp_training_exp", "luck_training_exp")
}

func redacted(r *reader, f map[string]any, name string) {
	r.utf(name)
	f[name] = "[redacted]"
}

func login(r *reader, f map[string]any) error {
	status := r.i8("status")
	f["status"] = status
	if status != 0 {
		return nil
	}
	fightInts(r, f, "zone_id", "attack", "defence", "agility", "luck", "damage", "armor")
	f["gp"] = r.long("gp")
	fightInts(r, f, "repute", "gold", "money", "money_lock", "medal", "score", "hide")
	f["fight_power"] = r.long("fight_power")
	apprenticeship(r, f)
	f["freezes_date"] = r.date("freezes_date")
	vip(r, f)
	f["system_date"] = r.date("system_date")
	f["can_take_vip_reward"] = r.boolean("can_take_vip_reward")
	fightInts(r, f, "option_on_off", "achievement_point")
	f["honor"] = r.utf("honor")
	f["total_game_time"] = r.i32("total_game_time")
	f["sex"] = r.boolean("sex")
	fightStrings(r, f, "style_and_colors", "skin")
	f["consortia_id"] = r.i32("consortia_id")
	f["consortia_name"] = r.utf("consortia_name")
	fightInts(r, f, "badge_id", "duty_level")
	f["duty_name"] = r.utf("duty_name")
	f["rights"] = r.i32("rights")
	f["chairman_name"] = r.utf("chairman_name")
	fightInts(r, f, "consortia_honor", "consortia_riches")
	f["bag_password_set"] = r.boolean("bag_password_set")
	redacted(r, f, "security_question_1")
	redacted(r, f, "security_question_2")
	f["security_attempts_left"] = r.i32("security_attempts_left")
	redacted(r, f, "login_name")
	f["nimbus"] = r.i32("nimbus")
	fightStrings(r, f, "pve_permission", "fight_library_mission")
	fightInts(r, f, "tutorial_progress", "received_box", "received_grade", "box_time_needed")
	f["last_spa_date"] = r.date("last_spa_date")
	f["shop_finally_gotten_time"] = r.date("shop_finally_gotten_time")
	fightInts(r, f, "used_offer", "daily_score", "daily_win_count", "daily_game_count")
	f["daily_league_first"] = r.boolean("daily_league_first")
	fightInts(r, f, "daily_league_last_score", "weekly_score", "weekly_game_count", "weekly_ranking")
	training(r, f)
	fightInts(r, f, "training_task_count", "training_count")
	f["training_task_date"] = r.date("training_task_date")
	f["old_player_valid_equipment"] = r.boolean("old_player_valid_equipment")
	fightInts(r, f, "bad_luck_number", "lucky_number")
	f["last_lucky_number_date"] = r.date("last_lucky_number_date")
	fightInts(r, f, "last_lucky_number", "totem_id", "accumulative_login_days", "accumulative_award_days")
	f["old_player"] = r.boolean("old_player")
	redacted(r, f, "client_instance")
	return r.err
}
