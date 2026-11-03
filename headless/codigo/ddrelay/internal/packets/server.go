package packets

import "ddbot/relay/internal/framing"

func decodeServer(r *reader, header framing.Header, fields map[string]any) error {
	switch header.Code {
	case 25:
		fightInts(r, fields, "uninterpreted_id", "operation")
		fields["success"] = r.boolean("success")
		fields["uninterpreted_flag"] = r.boolean("uninterpreted_flag")
		fields["message"] = r.utf("message")
		fields["attempts_left"] = r.i32("attempts_left")
		redacted(r, fields, "question_1")
		redacted(r, fields, "question_2")
	case 200:
		return captchaServer(r, fields)
	case 1:
		return login(r, fields)
	case 3:
		fields["kind"] = r.i32("kind")
		fields["message"] = r.utf("message")
	case 4:
		fields["uninterpreted_value"] = r.i32("uninterpreted_value")
	case 13:
		fields["available"] = r.boolean("available")
		fields["kind"] = r.i32("kind")
	case 64:
		return inventory(r, fields)
	case 71, 72:
		if header.Code == 72 {
			fields["big_bugle_type"] = r.i32("big_bugle_type")
		}
		fields["sender_id"] = r.i32("sender_id")
		fightStrings(r, fields, "sender", "message")
	case 70:
		return roomInvitation(r, fields)
	case 160:
		return socialServer(r, fields)
	case 74:
		return equipment(r, fields)
	case 75:
		fields["versions"] = fightIDs(r, "versions")
	case 92:
		vip(r, fields)
		fields["can_take_vip_reward"] = r.boolean("can_take_vip_reward")
	case 168:
		fields["shop_limits"] = goodsLimits(r)
		fields["consortia_id"] = r.i32("consortia_id")
		fields["consortia_shop_limits"] = goodsLimits(r)
		fields["uninterpreted_value"] = r.i32("uninterpreted_value")
	case 178:
		fields["quests"] = fightEntries(r, "quests", 37, func(f map[string]any) {
			f["quest_id"] = r.i32("quest_id")
			f["achieved"] = r.boolean("achieved")
			fightInts(r, f, "condition_1", "condition_2", "condition_3", "condition_4")
			f["complete_date"] = r.date("complete_date")
			fightInts(r, f, "repeat_left", "quality")
			f["exists"] = r.boolean("exists")
		})
	case 179:
		fields["quest_id"] = r.i32("quest_id")
	case 209:
		fields["subcommand"] = r.u8("subcommand")
		if fields["subcommand"] != uint8(1) {
			return ErrUnsupported
		}
		fields["update_flag"] = r.boolean("update_flag")
		fields["spirits"] = fightEntries(r, "spirits", 14, func(f map[string]any) {
			fightInts(r, f, "player_id", "spirit_id")
			f["value"] = r.utf("value")
			f["equip_place"] = r.i32("equip_place")
		})
	case 218:
		fightInts(r, fields, "player_id", "charm_gp")
		fields["gifts"] = fightEntries(r, "gifts", 8, func(f map[string]any) { fightInts(r, f, "template_id", "amount") })
	case 227:
		fields["enabled"] = r.boolean("enabled")
		fields["interface_id"] = r.i32("interface_id")
		fightFlags(r, fields, "approved", "minor")
	case 259:
		fields["recharged"] = r.boolean("recharged")
		fightInts(r, fields, "first_award_got", "week_award_got", "month_award_got", "first_condition", "week_condition", "month_condition")
		fields["rewards"] = fightEntries(r, "rewards", 8, func(f map[string]any) { fightInts(r, f, "id", "value") })
	case 277:
		fields["subcommand"] = r.u8("subcommand")
		if fields["subcommand"] != uint8(1) {
			return ErrUnsupported
		}
		fields["is_begin"] = r.boolean("is_begin")
	case 333:
		return dress(r, fields)
	case 343:
		fields["subcommand"] = r.u8("subcommand")
		if fields["subcommand"] != uint8(1) {
			return ErrUnsupported
		}
		fightFlags(r, fields, "notify", "activity_open")
		if fields["activity_open"] == true {
			fields["start_date"] = r.date("start_date")
			fields["end_date"] = r.date("end_date")
			fields["need_money"] = r.i32("need_money")
		}
	case 402:
		fields["subcommand"] = r.u8("subcommand")
		if fields["subcommand"] != uint8(5) {
			return ErrUnsupported
		}
		fields["collections"] = fightEntries(r, "collections", 19, func(f map[string]any) {
			fightInts(r, f, "id", "category")
			f["active_items"] = fightIDs(r, "active_items")
			f["end_time"] = r.date("end_time")
		})
	case 443:
		fields["show"] = r.boolean("show")
		fields["days"] = fightEntries(r, "days", 8, func(f map[string]any) { fightInts(r, f, "day", "state") })
	case 448:
		return battlePass(r, fields)
	case 5:
		fields["server_time"] = r.date("server_time")
	case 6:
		fields["player_id"] = header.ClientID
		fields["web_speed"] = r.i32("web_speed")
	case 10:
		return notice(r, fields)
	case 15:
		count := r.count("progress_length", 1)
		progress := make([]int8, 0, count)
		for i := 0; i < count && r.err == nil; i++ {
			progress = append(progress, r.i8("progress"))
		}
		fields["tutorial_progress"] = progress
	case 18:
		fields["player"] = scenePlayer(r, header.ClientID)
	case 19:
		fields["sender_id"] = header.ClientID
		fields["zone_id"] = r.i32("zone_id")
		fields["channel"] = r.i8("channel")
		fields["team"] = r.boolean("team")
		fields["sender"] = r.utf("sender")
		fields["message"] = r.utf("message")
	case 21:
		fields["player_id"] = header.ClientID
	case 34:
		count := r.count("honor_count", 2)
		honors := make([]string, 0, count)
		for i := 0; i < count && r.err == nil; i++ {
			honors = append(honors, r.utf("honor"))
		}
		fields["honors"] = honors
	case 38:
		for _, name := range []string{"money", "money_lock", "medal", "score", "gold", "gift", "bad_luck_number", "hard_currency"} {
			fields[name] = r.i32(name)
		}
		fields["honor"] = r.long("honor")
		fields["damage_scores"] = r.long("damage_scores")
	case 68:
		fields["subcommand"] = r.u8("subcommand")
		if fields["subcommand"] != uint8(33) {
			return ErrUnsupported
		}
		fightInts(r, fields, "weapon_exp", "weapon_level", "clothes_exp", "clothes_level", "hat_exp", "hat_level")
	case 107:
		fields["subcommand"] = r.i32("subcommand")
		if fields["subcommand"] != int32(5) {
			return ErrUnsupported
		}
		fields["invite_number"] = r.utf("invite_number")
		fields["success_count"] = r.i32("success_count")
		fields["last_fb_time"] = r.date("last_fb_time")
		fightInts(r, fields, "reward_count_1", "reward_count_2", "reward_count_3", "reward_count_4", "server_id")
	case 128:
		fields["subcommand"] = r.u8("subcommand")
		if fields["subcommand"] != uint8(4) {
			return ErrUnsupported
		}
		fightInts(r, fields, "invitation_id", "uninterpreted_id_1")
		fields["uninterpreted_name"] = r.utf("uninterpreted_name")
		fields["uninterpreted_id_2"] = r.i32("uninterpreted_id_2")
		fields["inviter_name"] = r.utf("inviter_name")
		fields["consortia_id"] = r.i32("consortia_id")
		fields["consortia_name"] = r.utf("consortia_name")
	case 129:
		fields["subcommand"] = r.u8("subcommand")
		if fields["subcommand"] != uint8(13) {
			return ErrUnsupported
		}
		fields["invitation_id"] = r.i32("invitation_id")
		fields["success"] = r.boolean("success")
		fields["message"] = r.utf("message")
	case 145:
		return activitySystem(r, fields)
	case 155:
		return dailyActivity(r, fields)
	case 239:
		fields["marry_info_id"] = r.i32("marry_info_id")
		fields["present"] = r.boolean("present")
		if fields["present"] == true {
			fields["player_id"] = r.i32("player_id")
			fields["publish_equipment"] = r.boolean("publish_equipment")
			fields["introduction"] = r.utf("introduction")
		}
	case 67:
		fields["player_id"] = header.ClientID
		return playerInfo(r, fields)
	case 91:
		return fightServer(r, header, fields)
	case 94:
		return roomServer(r, header, fields)
	case 95:
		fields["necklace_exp"] = r.i32("necklace_exp")
		fields["necklace_exp_add"] = r.i32("necklace_exp_add")
	case 185, 186:
		return buffs(r, fields)
	case 228, 229:
		return achievementProgress(r, fields)
	case 231:
		return achievementDates(r, fields)
	case 258:
		return noviceActivity(r, fields)
	case 342:
		return limitedCharge(r, fields)
	case 405:
		return activities(r, fields)
	case 700:
		return ranked(r, fields)
	case 1800:
		scenePlayers(r, fields)
	default:
		return ErrUnsupported
	}

	return r.err
}
