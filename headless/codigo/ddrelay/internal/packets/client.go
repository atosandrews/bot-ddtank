package packets

import "ddbot/relay/internal/framing"

func decodeClient(r *reader, header framing.Header, fields map[string]any) error {
	switch header.Code {
	case 25:
		redacted(r, fields, "password")
		redacted(r, fields, "new_password")
		fields["operation"] = r.i32("operation")
		redacted(r, fields, "question_1")
		redacted(r, fields, "answer_1")
		redacted(r, fields, "question_2")
		redacted(r, fields, "answer_2")
	case 200:
		redacted(r, fields, "answer")
	case 1:
		fields["build"] = r.i32("build")
		fields["desktop_type"] = r.i32("desktop_type")
		fields["rsa_payload_bytes"] = len(r.data) - r.offset
		r.take("rsa_payload", len(r.data)-r.offset)
	case 4:
		fields["client_time"] = r.date("client_time")
		fields["flag"] = r.boolean("flag")
	case 15:
		kind := r.u8("kind")
		fields["kind"] = kind
		if kind != 1 && kind != 2 {
			return ErrUnsupported
		}
		fields["step"] = r.i32("step")
		if kind == 1 {
			fields["flag"] = r.boolean("flag")
		}
	case 16:
		fields["scene_id"] = r.i32("scene_id")
	case 19:
		fields["channel"] = r.i8("channel")
		fields["team"] = r.boolean("team")
		fields["sender"] = r.utf("sender")
		fields["message"] = r.utf("message")
	case 21, 168, 331, 352, 700:
		return nil
	case 49:
		fields["from_bag"] = r.i8("from_bag")
		fields["from_place"] = r.i32("from_place")
		fields["to_bag"] = r.i8("to_bag")
		fightInts(r, fields, "to_place", "count")
		fields["move_flag"] = r.boolean("move_flag")
	case 94:
		return roomClient(r, fields)
	case 108:
		fields["argument"] = r.i32("argument")
	case 124:
		fields["merge"] = r.boolean("merge")
		count := r.i32("count")
		fields["bag_type"] = r.i32("bag_type")
		if count < 0 || count > 256 {
			return ErrUnsupported
		}

		moves := make([]map[string]any, 0, count)
		for index := int32(0); index < count; index++ {
			moves = append(moves, map[string]any{"from_place": r.i32("from_place"), "to_place": r.i32("to_place")})
		}
		fields["moves"] = moves
	case 179:
		fightInts(r, fields, "quest_id", "reward_argument")
	case 75:
		fields["argument"] = r.i32("argument")
	case 82:
		fields["flag"] = r.boolean("flag")
	case 70:
		fields["player_id"] = r.i32("player_id")
	case 74:
		byID := r.boolean("by_id")
		fields["by_id"] = byID
		if byID {
			fields["player_id"] = r.i32("player_id")
		} else {
			fields["nickname"] = r.utf("nickname")
		}
	case 91:
		return fightClient(r, fields)
	case 129:
		subcommand := r.i32("subcommand")
		fields["subcommand"] = subcommand
		if subcommand != 13 {
			return ErrUnsupported
		}
		fields["invitation_id"] = r.i32("invitation_id")
	case 159:
		fields["argument_1"] = r.i32("argument_1")
		fields["argument_2"] = r.i32("argument_2")
	case 160:
		subcommand := r.u8("subcommand")
		fields["subcommand"] = subcommand
		if subcommand == 160 {
			fields["nickname"] = r.utf("nickname")
			fields["relation"] = r.i32("relation")
			fightFlags(r, fields, "flag_1", "flag_2")

			return r.err
		}
		if subcommand == 45 {
			return nil
		}
		if subcommand != 165 {
			return ErrUnsupported
		}
		fields["state"] = r.i32("state")
	case 161:
		fields["flag"] = r.boolean("flag")
		fields["number"] = r.i32("number")
	case 176:
		count := r.count("quest_count", 4)
		quests := make([]int32, 0, count)
		for i := 0; i < count && r.err == nil; i++ {
			quests = append(quests, r.i32("quest_id"))
		}
		fields["quest_ids"] = quests
	case 181:
		fields["argument_1"] = r.i32("argument_1")
		fields["argument_2"] = r.i32("argument_2")
		fields["argument_3"] = r.i32("argument_3")
	case 213:
		fields["argument"] = r.i32("argument")
	case 218:
		fields["gift_id"] = r.i32("gift_id")
	case 236:
		fields["flag"] = r.boolean("flag")
		fields["text"] = r.utf("text")
		fields["argument"] = r.i32("argument")
	case 251:
		fields["state"] = r.i32("state")
	case 266:
		fields["subcommand"] = r.i32("subcommand")
		if fields["subcommand"] != int32(2) {
			return ErrUnsupported
		}
	case 405:
		fields["kind"] = r.i32("kind")
	default:
		return ErrUnsupported
	}

	return r.err
}
