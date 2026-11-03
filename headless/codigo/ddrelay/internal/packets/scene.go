package packets

func scenePlayer(r *reader, id int32) map[string]any {
	fields := map[string]any{"player_id": id}
	fields["grade"] = r.i32("grade")
	fields["sex"] = r.boolean("sex")
	fields["nickname"] = r.utf("nickname")
	fields["vip_type"] = r.i8("vip_type")
	fields["vip_level"] = r.i32("vip_level")
	fields["consortia_name"] = r.utf("consortia_name")
	for _, name := range []string{"offer", "win_count", "total_count", "escape_count", "consortia_id", "repute"} {
		fields[name] = r.i32(name)
	}
	married := r.boolean("married")
	fields["married"] = married
	if married {
		fields["spouse_id"] = r.i32("spouse_id")
		fields["spouse_name"] = r.utf("spouse_name")
	}
	r.utf("login_name")
	fields["login_name"] = "[redacted]"
	fields["fight_power"] = r.long("fight_power")
	fields["apprenticeship_state"] = r.i32("apprenticeship_state")
	return fields
}

func scenePlayers(r *reader, fields map[string]any) {
	count := r.count("player_count", 57)
	players := make([]map[string]any, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		players = append(players, scenePlayer(r, r.i32("player_id")))
	}
	fields["player_count"] = count
	fields["players"] = players
}

func roomList(r *reader, fields map[string]any) error {
	subcommand := r.u8("subcommand")
	fields["subcommand"] = subcommand
	if subcommand != 9 {
		return ErrUnsupported
	}
	fields["room_total"] = r.i32("room_total")
	count := r.count("room_count", 24)
	rooms := make([]map[string]any, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		room := make(map[string]any)
		room["id"] = r.i32("id")
		for _, name := range []string{"type", "time_type", "player_count", "viewer_count", "max_viewers", "place_count"} {
			room[name] = r.i8(name)
		}
		room["locked"] = r.boolean("locked")
		room["map_id"] = r.i32("map_id")
		room["playing"] = r.boolean("playing")
		room["name"] = r.utf("name")
		room["game_mode"] = r.i8("game_mode")
		room["hard_level"] = r.i8("hard_level")
		room["level_limits"] = r.i32("level_limits")
		rooms = append(rooms, room)
	}
	fields["room_count"] = count
	fields["rooms"] = rooms
	return nil
}
