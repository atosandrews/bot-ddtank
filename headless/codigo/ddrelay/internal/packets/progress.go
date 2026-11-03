package packets

func achievementProgress(r *reader, fields map[string]any) error {
	count := r.count("record_count", 8)
	entries := make([]map[string]any, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		id := r.i32("record_id")
		total := r.i32("total")
		entries = append(entries, map[string]any{"record_id": id, "total": total})
	}
	fields["records"] = entries
	return r.err
}

func achievementDates(r *reader, fields map[string]any) error {
	count := r.count("achievement_count", 16)
	entries := make([]map[string]any, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		entry := make(map[string]any)
		for _, name := range []string{"id", "year", "month", "day"} {
			entry[name] = r.i32(name)
		}
		entries = append(entries, entry)
	}
	fields["achievements"] = entries
	return r.err
}

func noviceActivity(r *reader, fields map[string]any) error {
	kind := r.i32("subcommand")
	fields["subcommand"] = kind
	switch kind {
	case 0:
		fields["activity_type"] = r.i32("activity_type")
		fields["conditions"] = r.i32("conditions")
		fields["award_got"] = r.i32("award_got")
		fields["start_date"] = r.date("start_date")
		fields["end_date"] = r.date("end_date")
	case 1:
		fields["open"] = r.boolean("open")
	default:
		return ErrUnsupported
	}
	return r.err
}

func limitedCharge(r *reader, fields map[string]any) error {
	kind := r.i32("subcommand")
	fields["subcommand"] = kind
	switch kind {
	case 0:
		fields["open"] = r.boolean("open")
	case 1:
		count := r.count("user_count", 13)
		users := make([]map[string]any, 0, count)
		for i := 0; i < count && r.err == nil; i++ {
			user := make(map[string]any)
			user["user_id"] = r.i32("user_id")
			user["activity_id"] = r.i32("activity_id")
			user["activity_amount"] = r.i32("activity_amount")
			user["finished"] = r.boolean("finished")
			users = append(users, user)
		}
		fields["users"] = users
	default:
		return ErrUnsupported
	}
	return r.err
}

func buffs(r *reader, fields map[string]any) error {
	count := r.count("buff_count", 28)
	entries := make([]map[string]any, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		entry := make(map[string]any)
		entry["type"] = r.i32("type")
		entry["flag"] = r.boolean("flag")
		entry["date"] = r.date("date")
		for _, name := range []string{"argument_1", "argument_2", "argument_3", "argument_4"} {
			entry[name] = r.i32(name)
		}
		entries = append(entries, entry)
	}
	fields["buffs"] = entries
	return r.err
}
