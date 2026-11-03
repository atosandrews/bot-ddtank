package packets

func activities(r *reader, fields map[string]any) error {
	kind := r.i32("kind")
	fields["kind"] = kind
	if kind == 0 {
		return nil
	}
	if kind < 1 || kind > 3 {
		return ErrUnsupported
	}
	count := r.count("activity_count", 10)
	entries := make([]map[string]any, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		entry := make(map[string]any)
		entry["id"] = r.utf("activity_id")
		statuses := r.count("status_count", 8)
		status := make([]map[string]any, 0, statuses)
		for j := 0; j < statuses && r.err == nil; j++ {
			id := r.i32("status_id")
			value := r.i32("status_value")
			status = append(status, map[string]any{"id": id, "value": value})
		}
		entry["statuses"] = status
		gifts := r.count("gift_count", 14)
		giftEntries := make([]map[string]any, 0, gifts)
		for j := 0; j < gifts && r.err == nil; j++ {
			gift := make(map[string]any)
			gift["id"] = r.utf("gift_id")
			gift["times"] = r.i32("times")
			gift["all_gift_get_times"] = r.i32("all_gift_get_times")
			gift["remaining_times"] = r.i32("remaining_times")
			giftEntries = append(giftEntries, gift)
		}
		entry["gifts"] = giftEntries
		entries = append(entries, entry)
	}
	fields["activity_count"] = count
	fields["activities"] = entries
	return nil
}
