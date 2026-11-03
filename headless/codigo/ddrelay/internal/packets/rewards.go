package packets

func goodsLimits(r *reader) []map[string]any {
	return fightEntries(r, "goods_limits", 8, func(f map[string]any) { fightInts(r, f, "goods_id", "limit_count") })
}

func vip(r *reader, f map[string]any) {
	f["vip_type"] = r.i8("vip_type")
	fightInts(r, f, "vip_level", "vip_exp")
	f["vip_expire_day"] = r.date("vip_expire_day")
	f["last_date"] = r.date("last_date")
	f["vip_next_level_days_needed"] = r.i32("vip_next_level_days_needed")
}

func dress(r *reader, f map[string]any) error {
	sub := r.u8("subcommand")
	f["subcommand"] = sub
	switch sub {
	case 1:
		f["models"] = fightEntries(r, "models", 8, func(m map[string]any) {
			m["model_id"] = r.i32("model_id")
			m["items"] = fightEntries(r, "items", 8, func(i map[string]any) { fightInts(r, i, "item_id", "template_id") })
		})
	case 2:
		f["current_model"] = r.i32("current_model")
	default:
		return ErrUnsupported
	}
	return r.err
}

func battlePass(r *reader, f map[string]any) error {
	f["is_premium"] = r.boolean("is_premium")
	fightInts(r, f, "current_level", "current_exp", "premium_expire_days", "exp_buff_bonus_pct", "exp_buff_remaining_minutes", "season_id")
	f["season_end_date"] = r.utf("season_end_date")
	fightInts(r, f, "total_levels", "exp_per_level", "featured_slot")
	f["collected"] = fightEntries(r, "collected", 8, func(e map[string]any) { fightInts(r, e, "level", "track_type") })
	return r.err
}
