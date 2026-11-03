package packets

func fightCardResult(r *reader, fields map[string]any) error {
	fields["system_selected"] = r.boolean("system_selected")
	index := r.i8("card_index")
	fields["card_index"] = index
	if index == 50 {
		return r.err
	}

	fightInts(r, fields, "template_id", "count")
	if r.err == nil && r.offset < len(r.data) {
		fields["vip"] = r.boolean("vip")
	}

	return r.err
}
