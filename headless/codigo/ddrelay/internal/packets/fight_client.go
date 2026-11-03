package packets

func fightClient(r *reader, f map[string]any) error {
	switch fightCommand(r, f) {
	case 2:
		fightInts(r, f, "x", "y", "force", "angle")
	case 3:
		fightInts(r, f, "argument_1", "argument_2", "argument_3", "argument_4")
	case 10:
		fightInts(r, f, "x", "y")
		f["move_flag"] = r.boolean("move_flag")
	case 12:
		f["argument"] = r.i8("argument")
	case 19:
		fightInts(r, f, "argument_1", "argument_2", "argument_3", "argument_4", "argument_5")
	case 32:
		f["bag_type"] = r.i8("bag_type")
		fightInts(r, f, "place", "template_id")
	case 98, 114, 130:
		f["card_index"] = r.i8("card_index")
	case 116:
		f["ready"] = r.boolean("ready")
	case 119:
		f["argument"] = r.i32("argument")
		fightFlags(r, f, "flag_1", "flag_2")
	case 7:
		f["direction"] = r.i32("direction")
	case 9:
		f["moving"] = r.boolean("moving")
		f["move_type"] = r.i8("move_type")
		fightInts(r, f, "x", "y")
		f["direction"] = r.i8("direction")
		f["is_living"] = r.boolean("is_living")
		f["turn_number"] = int16(r.u16("turn_number"))
	case 15:
		f["stunt_type"] = r.i8("stunt_type")
	case 16:
		f["progress"] = r.i32("progress")
	case 40:
		return r.err
	case 96:
		started := r.boolean("started")
		f["started"] = started
		if started {
			f["shot_type"] = r.i8("shot_type")
		}
	case 149:
		f["enabled"] = r.boolean("enabled")
	default:
		return ErrUnsupported
	}
	return r.err
}
