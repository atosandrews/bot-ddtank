package packets

func socialServer(r *reader, f map[string]any) error {
	sub := r.u8("subcommand")
	f["subcommand"] = sub

	switch sub {
	case 160:
		f["success"] = r.boolean("success")
		if f["success"] == false {
			return r.err
		}

		f["player_id"] = r.i32("player_id")
		f["nickname"] = r.utf("nickname")
		f["vip_type"] = r.i8("vip_type")
		f["vip_level"] = r.i32("vip_level")
		f["sex"] = r.boolean("sex")
		fightStrings(r, f, "style", "colors", "skin")
		fightInts(r, f, "state", "grade", "hide")
		f["consortia_name"] = r.utf("consortia_name")
		fightInts(r, f, "total_count", "escape_count", "win_count", "offer", "repute", "relation")
		r.utf("login_name")
		f["login_name"] = "[redacted]"
		f["nimbus"] = r.i32("nimbus")
		f["fight_power"] = r.long("fight_power")
		fightInts(r, f, "apprenticeship_state", "master_id")
		f["apprentices"] = r.utf("apprentices")
		f["graduates_count"] = r.i32("graduates_count")
		f["master_honor"] = r.utf("master_honor")
		f["achievement_points"] = r.i32("achievement_points")
		f["honor"] = r.utf("honor")
		f["married"] = r.boolean("married")
	case 166:
		f["argument"] = r.i32("argument")
		f["nickname"] = r.utf("nickname")
		f["same_city"] = r.boolean("same_city")
	default:
		return ErrUnsupported
	}

	return r.err
}

func roomInvitation(r *reader, f map[string]any) error {
	fightInts(r, f, "player_id", "room_id", "map_id")

	for _, name := range []string{"second_type", "game_mode", "difficulty", "level_limits"} {
		f[name] = r.i8(name)
	}

	f["nickname"] = r.utf("nickname")
	f["vip"] = r.boolean("vip")
	f["vip_level"] = r.i32("vip_level")
	f["room_name"] = r.utf("room_name")
	r.utf("password")
	f["password"] = "[redacted]"
	f["barrier_number"] = r.i32("barrier_number")

	return r.err
}
