package packets

import (
	"reflect"
	"testing"

	"ddbot/relay/internal/capture"
)

func TestFightLifecycle(t *testing.T) {
	date := []byte{7, 234, 10, 14, 18, 30, 0}
	resources := join(ints(1, 2), utf("resource.swf"), utf("Example"))
	resourceFields := []map[string]any{{"type": int32(2), "path": "resource.swf", "class_path": "Example"}}
	tests := []struct {
		name string
		sub  byte
		data []byte
		want map[string]any
	}{
		{"resources", 67, resources, map[string]any{"resources": resourceFields}},
		{"game load", 103, join(ints(30, 100), resources, ints(1), utf("pet"), utf("spark")), map[string]any{"max_time": int32(30), "map_id": int32(100), "resources": resourceFields, "pet_resources": []map[string]any{{"picture": "pet", "effect": "spark"}}}},
		{"start", 99, join(ints(1, 10, 20, 30, -1, 40, 50, 2, 3, 4, 5, 1, 6, 7, 2, 8, 9), []byte{1, 0, 1, 0}, ints(1), utf("speed"), utf("5"), date), map[string]any{"players": []map[string]any{{"living_id": int32(10), "x": int32(20), "y": int32(30), "direction": int32(-1), "hp": int32(40), "max_hp": int32(50), "team": int32(2), "refinery_level": int32(3), "power_ratio": int32(4), "dander": int32(5), "buffs": []map[string]any{{"id": int32(6), "data": int32(7)}}, "out_turn_buff_ids": []int32{8, 9}, "frozen": true, "hidden": false, "no_hole": true, "state_flag": false, "properties": []map[string]any{{"name": "speed", "value": "5"}}}}, "server_time": "2026-10-14T18:30:00"}},
		{"beat", 58, join(utf("attack"), ints(2, 1, 20, 80, 30, 4, 2, 40, 60, 50, 6)), map[string]any{"living_id": int32(77), "action": "attack", "targets": []map[string]any{{"target_id": int32(1), "damage": int32(20), "hp": int32(80), "dander": int32(30), "attack_effect": int32(4)}, {"target_id": int32(2), "damage": int32(40), "hp": int32(60), "dander": int32(50), "attack_effect": int32(6)}}}},
		{"mission", 113, join(ints(7), utf("Mission"), utf("Win"), utf("Lose"), utf("Description"), utf("config"), ints(2, 1, 3, 4, 5, 6, 7), utf("picture")), map[string]any{"mission_id": int32(7), "name": "Mission", "success": "Win", "failure": "Lose", "description": "Description", "mission_config": "config", "total_missions": int32(2), "mission_index": int32(1), "total_value_1": int32(3), "total_value_2": int32(4), "total_value_3": int32(5), "total_value_4": int32(6), "try_again": int32(7), "picture": "picture"}},
		{"mission end", 112, join(ints(1), []byte{1}, utf("next"), []byte{0}, ints(1, 2, 3, 4, 5), []byte{1}, ints(6, 7), []byte{0, 1}), map[string]any{"take_card_type": int32(1), "has_next_mission": true, "next_picture": "next", "can_enter_final": false, "players": []map[string]any{{"player_id": int32(2), "level": int32(3), "treatment": int32(4), "gained_gp": int32(5), "winner": true, "card_count": int32(6), "result_argument": int32(7), "has_level_again": false, "has_card_get": true}}}},
		{"all over", 115, join(ints(1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14), []byte{1}, ints(2), utf("one"), utf("two")), map[string]any{"players": []map[string]any{{"player_id": int32(1), "kill_gp": int32(2), "hurt_gp": int32(3), "fight_gp": int32(4), "cure_gp": int32(5), "vip_gp": int32(6), "spouse_gp": int32(7), "server_gp": int32(8), "apprentice_online_gp": int32(9), "apprentice_team_gp": int32(10), "double_card_gp": int32(11), "guild_skill_gp": int32(12), "lucky_gp": int32(13), "gained_gp": int32(14), "winner": true}}, "npc_movies": []string{"one", "two"}}},
		{"style", 134, join([]byte{0}, ints(1), utf("style"), ints(2), []byte{1}, utf("skin"), utf("colors"), ints(3), utf("pet")), map[string]any{"kind": int8(0), "players": []map[string]any{{"style": "style", "hide": int32(2), "sex": true, "skin": "skin", "colors": "colors", "player_id": int32(3), "pet_style": "pet"}}}},
		{"pet style", 134, join([]byte{1}, ints(3), []byte{1}), map[string]any{"kind": int8(1), "player_id": int32(3), "use_pet_style": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := join([]byte{tt.sub}, tt.data)
			tt.want["subcommand"] = uint8(tt.sub)
			tt.want["event"] = fightNames[tt.sub]
			assertFight(t, decodeFight(capture.ServerToClient, body), body, tt.want)
			assertFightTruncations(t, capture.ServerToClient, body, nil)
		})
	}
	body := join([]byte{112}, ints(1), []byte{0, 0}, ints(0, 1), utf("outro"))
	got := decodeFight(capture.ServerToClient, body)
	if got.Status != "decoded" || !reflect.DeepEqual(got.Fields["npc_movies"], []string{"outro"}) {
		t.Fatalf("movie suffix = %#v", got)
	}
}

func TestFightCreateConditionalPlayers(t *testing.T) {
	for _, equipped := range []bool{false, true} {
		t.Run(map[bool]string{false: "minimal", true: "equipped married with pet"}[equipped], func(t *testing.T) {
			player := join(ints(1), utf("Zone"), ints(2), utf("Player"), []byte{0, 1}, ints(3), []byte{1}, ints(4), utf("style"), utf("colors"), utf("skin"), ints(5, 6))
			if equipped {
				player = join(player, ints(7, 8), utf("weapon"), []byte{7, 234, 10, 14, 18, 30, 0})
			} else {
				player = join(player, ints(0))
			}
			player = join(player, ints(9, 10), []byte{1}, ints(11), utf("Guild"), ints(12, 13, 14, 15, 16, 1, 17, 18, 19), utf("Mentor"), ints(20), utf("Honor"), ints(21), []byte{1}, ints(22))
			if equipped {
				player = join(player, []byte{1}, ints(23), utf("Spouse"))
			} else {
				player = join(player, []byte{0})
			}
			player = join(player, ints(24, 25, 26, 27, 28, 29, 30, 31, 32))
			if equipped {
				player = join(player, ints(1, 33, 34, 35), utf("Pet"), ints(36, 37, 1, 38, 39))
			} else {
				player = join(player, ints(0))
			}
			body := join([]byte{101}, ints(1, 2, 3, 1), player)
			got := decodeFight(capture.ServerToClient, body)
			if got.Status != "decoded" || got.Err != nil || got.Consumed != len(body) {
				t.Fatalf("create = %#v", got)
			}
			p := got.Fields["players"].([]map[string]any)[0]
			want := map[string]any{"zone_id": int32(1), "zone_name": "Zone", "player_id": int32(2), "nickname": "Player", "viewer": false, "vip_type": int8(1), "vip_level": int32(3), "sex": true, "hide": int32(4), "style": "style", "colors": "colors", "skin": "skin", "grade": int32(5), "repute": int32(6), "weapon_id": int32(0), "deputy_weapon_id": int32(9), "nimbus": int32(10), "show_guild": true, "guild_id": int32(11), "guild_name": "Guild", "badge_id": int32(12), "unassigned_int_1": int32(13), "unassigned_int_2": int32(14), "wins": int32(15), "total_games": int32(16), "fight_power": int64(4294967313), "apprenticeship_state": int32(18), "master_id": int32(19), "master_or_apprentices": "Mentor", "achievement_points": int32(20), "honor": "Honor", "offer": int32(21), "daily_league_first": true, "daily_league_last_score": int32(22), "married": equipped, "gm_experience_hundredths": int32(24), "launcher_experience_hundredths": int32(25), "gm_offer_hundredths": int32(26), "launcher_offer_hundredths": int32(27), "gm_riches_hundredths": int32(28), "launcher_riches_hundredths": int32(29), "team": int32(30), "living_id": int32(31), "max_hp": int32(32), "pets": []map[string]any{}}
			if equipped {
				want["weapon_id"] = int32(7)
				want["refinery_level"] = int32(8)
				want["weapon_argument"] = "weapon"
				want["weapon_date"] = "2026-10-14T18:30:00"
				want["spouse_id"] = int32(23)
				want["spouse_name"] = "Spouse"
				want["pets"] = []map[string]any{{"place": int32(33), "template_id": int32(34), "id": int32(35), "name": "Pet", "user_id": int32(36), "level": int32(37), "skills": []map[string]any{{"slot": int32(38), "skill_id": int32(39)}}}}
			}
			if !reflect.DeepEqual(p, want) {
				t.Fatalf("player = %#v\nwant = %#v", p, want)
			}
			assertFightTruncations(t, capture.ServerToClient, body, nil)
			pair := join([]byte{101}, ints(1, 2, 3, 2), player, player)
			got = decodeFight(capture.ServerToClient, pair)
			if got.Status != "decoded" || !reflect.DeepEqual(got.Fields["players"], []map[string]any{want, want}) {
				t.Fatalf("pair = %#v", got)
			}
		})
	}
}
