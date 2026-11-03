package packets

import "ddbot/relay/internal/framing"

func roomClient(r *reader, f map[string]any) error {
	sub := r.i32("subcommand")
	f["subcommand"] = sub
	switch sub {
	case 0:
		f["room_type"] = r.i8("room_type")
		f["time_type"] = r.i8("time_type")
		f["name"] = r.utf("name")
		redacted(r, f, "password")
	case 1:
		f["login_flag"] = r.boolean("login_flag")
		fightInts(r, f, "room_type", "room_id")
		if f["room_id"] == int32(-1) {
			f["argument"] = r.i32("argument")
			redacted(r, f, "password")
		}
	case 2:
		f["map_id"] = r.i32("map_id")
		f["room_type"] = r.i8("room_type")
		redacted(r, f, "password")
		f["name"] = r.utf("name")
		f["time_type"] = r.i8("time_type")
		f["difficulty"] = r.i8("difficulty")
		f["level_limits"] = r.i32("level_limits")
		fightFlags(r, f, "setup_flag_1", "setup_flag_2")
	case 3:
		f["place"] = r.i8("place")
	case 6:
		f["team"] = r.i8("team")
	case 10:
		f["place"] = r.i8("place")
		f["state"] = r.i32("state")
		f["place_flag"] = r.boolean("place_flag")
		f["argument"] = r.i32("argument")
	case 12:
		f["game_style"] = r.i32("game_style")
	case 18:
		f["argument"] = r.i32("argument")
	case 5, 7, 11:
	case 9:
		fightInts(r, f, "list_type", "filter")
		if f["list_type"] == int32(2) && f["filter"] == int32(-2) {
			fightInts(r, f, "argument_1", "argument_2")
		}
	case 15:
		f["state"] = r.i8("state")
	default:
		return ErrUnsupported
	}
	return r.err
}

func roomServer(r *reader, h framing.Header, f map[string]any) error {
	if len(r.data)-r.offset > 0 && r.data[r.offset] == 9 {
		return roomList(r, f)
	}
	sub := r.u8("subcommand")
	f["subcommand"] = sub
	switch sub {
	case 0:
		f["room_id"] = r.i32("room_id")
		for _, name := range []string{"room_type", "difficulty", "time_type", "player_count", "viewer_count", "place_count"} {
			f[name] = r.i8(name)
		}
		f["locked"] = r.boolean("locked")
		f["map_id"] = r.i32("map_id")
		f["started"] = r.boolean("started")
		f["name"] = r.utf("name")
		f["game_mode"] = r.i8("game_mode")
		f["level_limits"] = r.i32("level_limits")
		fightFlags(r, f, "cross_zone", "within_league_time")
	case 1:
		f["success"] = r.boolean("success")
	case 2:
		f["open_boss"] = r.boolean("open_boss")
		f["picture"] = r.utf("picture")
		f["map_id"] = r.i32("map_id")
		f["room_type"] = r.i8("room_type")
		redacted(r, f, "password")
		f["name"] = r.utf("name")
		f["time_type"] = r.i8("time_type")
		f["difficulty"] = r.i8("difficulty")
		f["level_limits"] = r.i32("level_limits")
		f["cross_zone"] = r.boolean("cross_zone")
	case 4:
		return roomPlayer(r, h, f)
	case 6:
		f["player_id"] = h.ClientID
		f["team"] = r.i8("team")
		f["place"] = r.i8("place")
	case 14:
		f["room_id"] = r.i32("room_id")
	case 5:
		f["player_id"] = h.ClientID
		f["zone_id"] = r.i32("zone_id")
	case 10:
		places := make([]int32, 10)
		for i := range places {
			places[i] = r.i32("place_state")
		}
		f["places"] = places
	case 11, 13:
	case 12:
		f["game_style"] = r.i8("game_style")
	case 15:
		states := make([]int8, 8)
		for i := range states {
			states[i] = r.i8("player_state")
		}
		f["player_states"] = states
	default:
		return ErrUnsupported
	}
	return r.err
}

func roomPlayer(r *reader, h framing.Header, f map[string]any) error {
	f["header_player_id"] = h.ClientID
	f["room_flag"] = r.boolean("room_flag")
	f["place"] = r.i8("place")
	f["team"] = r.i8("team")
	f["first_in"] = r.boolean("first_in")
	fightInts(r, f, "grade", "offer", "hide", "repute", "uninterpreted_value", "zone_id")
	if r.err != nil || r.offset == len(r.data) {
		f["layout"] = "common_prefix"
		return r.err
	}
	f["layout"] = "full_player"
	f["player_id"] = r.i32("player_id")
	f["nickname"] = r.utf("nickname")
	f["vip_type"] = r.i8("vip_type")
	f["vip_level"] = r.i32("vip_level")
	f["sex"] = r.boolean("sex")
	fightStrings(r, f, "style", "colors", "skin")
	fightInts(r, f, "weapon_id", "deputy_weapon_id", "consortia_id")
	f["consortia_name"] = r.utf("consortia_name")
	fightInts(r, f, "badge_id", "win_count", "total_count", "escape_count", "uninterpreted_value_2", "uninterpreted_value_3")
	married := r.boolean("married")
	f["married"] = married
	if married {
		f["spouse_id"] = r.i32("spouse_id")
		f["spouse_name"] = r.utf("spouse_name")
	}
	redacted(r, f, "login_name")
	f["nimbus"] = r.i32("nimbus")
	f["fight_power"] = r.long("fight_power")
	apprenticeship(r, f)
	f["daily_league_first"] = r.boolean("daily_league_first")
	f["daily_league_last_score"] = r.i32("daily_league_last_score")
	f["pets"] = fightEntries(r, "pets", 26, func(p map[string]any) {
		fightInts(r, p, "place", "template_id", "id")
		p["name"] = r.utf("name")
		fightInts(r, p, "user_id", "level")
		p["skills"] = fightEntries(r, "skills", 8, func(s map[string]any) { fightInts(r, s, "slot", "id") })
	})
	return r.err
}
