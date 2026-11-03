package packets

var fightNames = map[uint8]string{
	3: "BLAST", 10: "MOVESTOP", 11: "HEALTH", 12: "SKIPNEXT", 19: "CHANGEBALL",
	80: "LIVING_SHOW_BLOOD", 89: "SHOW_CARDS", 98: "TAKE_CARD", 114: "PAYMENT_TAKE_CARD",
	116: "GAME_MISSION_PREPARE", 119: "GAME_MISSION_TRY_AGAIN", 130: "BOSS_TAKE_CARD",
	2: "FIRE", 4: "ROUND_ONE_END", 6: "TURN", 7: "DIRECTION", 9: "MOVESTART",
	14: "DANDER", 15: "STUNT", 16: "LOAD", 20: "CHANGE_BALL", 32: "PROP",
	40: "AIRPLANE", 41: "PLAYER_PROPERTY", 46: "CHANGE_SHOOT_COUNT", 55: "LIVING_MOVETO", 56: "LIVING_FALLING",
	58: "LIVING_BEAT", 59: "LIVING_SAY", 64: "ADD_LIVING", 67: "LOAD_RESOURCE",
	96: "FIRE_TAG", 99: "START_GAME", 101: "GAME_CREATE", 103: "GAME_LOAD",
	104: "BARRIER_INFO", 112: "MISSION_OVE", 113: "GAME_MISSION_INFO",
	115: "GAME_ALL_MISSION_OVER", 131: "SYNC_LIFETIME", 134: "TEMP_STYLE",
	149: "GAME_TRUSTEESHIP", 241: "WIND_PIC", 252: "PSYCHIC_SET",
}

func fightCommand(r *reader, f map[string]any) uint8 {
	sub := r.u8("subcommand")
	f["subcommand"] = sub
	name, ok := fightNames[sub]
	if !ok {
		name = "UNKNOWN"
	}
	f["event"] = name
	return sub
}

func fightInts(r *reader, f map[string]any, names ...string) {
	for _, name := range names {
		f[name] = r.i32(name)
	}
}

func fightStrings(r *reader, f map[string]any, names ...string) {
	for _, name := range names {
		f[name] = r.utf(name)
	}
}

func fightFlags(r *reader, f map[string]any, names ...string) {
	for _, name := range names {
		f[name] = r.boolean(name)
	}
}

func fightEntries(r *reader, name string, minBytes int, read func(map[string]any)) []map[string]any {
	count := r.count(name+".count", minBytes)
	entries := make([]map[string]any, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		entry := make(map[string]any)
		read(entry)
		entries = append(entries, entry)
	}
	return entries
}

func fightProperties(r *reader) []map[string]any {
	return fightEntries(r, "properties", 4, func(f map[string]any) {
		fightStrings(r, f, "name", "value")
	})
}

func fightIDs(r *reader, name string) []int32 {
	count := r.count(name+".count", 4)
	ids := make([]int32, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		ids = append(ids, r.i32(name))
	}
	return ids
}

func fightWind(r *reader, f map[string]any) {
	f["wind_tenths"] = r.i32("wind_tenths")
	fightWindDisplay(r, f)
}

func fightWindDisplay(r *reader, f map[string]any) {
	f["wind_display_flag"] = r.boolean("wind_display_flag")
	f["wind_display_bytes"] = []int8{r.i8("wind_display_1"), r.i8("wind_display_2"), r.i8("wind_display_3")}
}

func fightResources(r *reader) []map[string]any {
	return fightEntries(r, "resources", 8, func(f map[string]any) {
		f["type"] = r.i32("type")
		fightStrings(r, f, "path", "class_path")
	})
}
