package packets

func activitySystem(r *reader, f map[string]any) error {
	sub := r.u8("subcommand")
	f["subcommand"] = sub
	switch sub {
	case 32:
		f["event"] = "CATCHBEAST_BEGIN"
		f["is_begin"] = r.boolean("is_begin")
	case 74:
		f["event"] = "DDPLAY_BEGIN"
		f["open"] = r.boolean("open")
		f["begin_date"] = r.date("begin_date")
		f["end_date"] = r.date("end_date")
		fightInts(r, f, "money", "exchange_fold")
	default:
		return ErrUnsupported
	}
	return r.err
}

func dailyActivity(r *reader, f map[string]any) error {
	f["subcommand"] = r.u8("subcommand")
	if f["subcommand"] != uint8(1) {
		return ErrUnsupported
	}
	f["activities"] = fightEntries(r, "activities", 8, func(e map[string]any) { fightInts(r, e, "id", "completed_count") })
	f["buttons"] = fightEntries(r, "buttons", 8, func(e map[string]any) { fightInts(r, e, "id", "state") })
	f["sessions"] = fightEntries(r, "sessions", 8, func(e map[string]any) { fightInts(r, e, "id", "value") })
	f["activity_value"] = r.i32("activity_value")
	f["find_back"] = fightEntries(r, "find_back", 6, func(e map[string]any) {
		e["id"] = r.i32("id")
		fightFlags(r, e, "flag_1", "flag_2")
	})
	f["speed_activities"] = r.utf("speed_activities")
	return r.err
}
