package packets

func ranked(r *reader, fields map[string]any) error {
	fields["season_id"] = r.i32("season_id")
	fields["elo_tier"] = r.u8("elo_tier")
	fields["elo_division"] = r.u8("elo_division")
	for _, name := range []string{"ranked_pdl", "total_wins", "total_matches"} {
		fields[name] = r.i32(name)
	}
	fields["win_streak"] = int16(r.u16("win_streak"))
	fields["max_streak"] = int16(r.u16("max_streak"))
	for _, name := range []string{"icon_unlock", "title_unlock", "elo_reward_bits"} {
		fields[name] = r.long(name)
	}
	fields["selected_icon"] = r.u8("selected_icon")
	fields["selected_title"] = r.u8("selected_title")
	fields["season_active"] = r.boolean("season_active")
	fields["season_remaining_seconds"] = r.i32("season_remaining_seconds")
	fields["division_floor"] = r.i32("division_floor")
	fields["next_division_target"] = r.i32("next_division_target")
	count := r.count("reward_count", 9)
	rewards := make([]map[string]any, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		reward := make(map[string]any)
		reward["elo_tier"] = r.u8("elo_tier")
		reward["template_id"] = r.i32("template_id")
		reward["count"] = r.i32("count")
		rewards = append(rewards, reward)
	}
	fields["rewards"] = rewards
	return r.err
}
