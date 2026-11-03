package packets

func fightMissionOver(r *reader, f map[string]any) error {
	f["take_card_type"] = r.i32("take_card_type")
	next := r.boolean("has_next_mission")
	f["has_next_mission"] = next
	if next {
		f["next_picture"] = r.utf("next_picture")
	}
	f["can_enter_final"] = r.boolean("can_enter_final")
	f["players"] = fightEntries(r, "players", 27, func(e map[string]any) {
		fightInts(r, e, "player_id", "level", "treatment", "gained_gp")
		e["winner"] = r.boolean("winner")
		fightInts(r, e, "card_count", "result_argument")
		fightFlags(r, e, "has_level_again", "has_card_get")
	})
	if r.err == nil && r.offset < len(r.data) {
		f["npc_movies"] = fightMovieNames(r)
	}
	return r.err
}

func fightAllOver(r *reader, f map[string]any) error {
	f["players"] = fightEntries(r, "players", 57, func(e map[string]any) {
		fightInts(r, e, "player_id", "kill_gp", "hurt_gp", "fight_gp", "cure_gp", "vip_gp", "spouse_gp", "server_gp", "apprentice_online_gp", "apprentice_team_gp", "double_card_gp", "guild_skill_gp", "lucky_gp", "gained_gp")
		e["winner"] = r.boolean("winner")
	})
	f["npc_movies"] = fightMovieNames(r)
	return r.err
}

func fightMovieNames(r *reader) []string {
	count := r.count("npc_movies.count", 2)
	names := make([]string, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		names = append(names, r.utf("npc_movie"))
	}
	return names
}
