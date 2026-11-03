package packets

func fightCreate(r *reader, f map[string]any) error {
	fightInts(r, f, "room_type", "game_mode", "time_type")
	f["players"] = fightEntries(r, "players", 150, func(e map[string]any) {
		e["zone_id"] = r.i32("zone_id")
		e["zone_name"] = r.utf("zone_name")
		e["player_id"] = r.i32("player_id")
		e["nickname"] = r.utf("nickname")
		e["viewer"] = r.boolean("viewer")
		e["vip_type"] = r.i8("vip_type")
		e["vip_level"] = r.i32("vip_level")
		e["sex"] = r.boolean("sex")
		e["hide"] = r.i32("hide")
		fightStrings(r, e, "style", "colors", "skin")
		fightInts(r, e, "grade", "repute")
		weapon := r.i32("weapon_id")
		e["weapon_id"] = weapon
		if weapon != 0 {
			e["refinery_level"] = r.i32("refinery_level")
			e["weapon_argument"] = r.utf("weapon_argument")
			e["weapon_date"] = r.date("weapon_date")
		}
		fightInts(r, e, "deputy_weapon_id", "nimbus")
		e["show_guild"] = r.boolean("show_guild")
		e["guild_id"] = r.i32("guild_id")
		e["guild_name"] = r.utf("guild_name")
		fightInts(r, e, "badge_id", "unassigned_int_1", "unassigned_int_2", "wins", "total_games")
		e["fight_power"] = r.long("fight_power")
		fightInts(r, e, "apprenticeship_state", "master_id")
		e["master_or_apprentices"] = r.utf("master_or_apprentices")
		e["achievement_points"] = r.i32("achievement_points")
		e["honor"] = r.utf("honor")
		e["offer"] = r.i32("offer")
		e["daily_league_first"] = r.boolean("daily_league_first")
		e["daily_league_last_score"] = r.i32("daily_league_last_score")
		married := r.boolean("married")
		e["married"] = married
		if married {
			e["spouse_id"] = r.i32("spouse_id")
			e["spouse_name"] = r.utf("spouse_name")
		}
		fightInts(r, e, "gm_experience_hundredths", "launcher_experience_hundredths", "gm_offer_hundredths", "launcher_offer_hundredths", "gm_riches_hundredths", "launcher_riches_hundredths", "team", "living_id", "max_hp")
		e["pets"] = fightEntries(r, "pets", 26, func(p map[string]any) {
			fightInts(r, p, "place", "template_id", "id")
			p["name"] = r.utf("name")
			fightInts(r, p, "user_id", "level")
			p["skills"] = fightEntries(r, "skills", 8, func(s map[string]any) { fightInts(r, s, "slot", "skill_id") })
		})
	})
	return r.err
}
