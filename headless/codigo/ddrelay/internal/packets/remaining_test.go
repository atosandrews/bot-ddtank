package packets

import (
	"bytes"
	"reflect"
	"testing"

	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
)

func TestRemainingBodies(t *testing.T) {
	date := []byte{7, 234, 10, 14, 12, 34, 56}
	dateText := "2026-10-14T12:34:56"
	tests := []struct {
		name   string
		client bool
		code   int16
		body   []byte
		want   map[string]any
	}{
		{"move goods", true, 49, join([]byte{1}, ints(2), []byte{3}, ints(4, 5), []byte{1}), map[string]any{"from_bag": int8(1), "from_place": int32(2), "to_bag": int8(3), "to_place": int32(4), "count": int32(5), "move_flag": true}},
		{"temporary bag", true, 108, ints(2), map[string]any{"argument": int32(2)}},
		{"goods request", true, 168, nil, map[string]any{}},
		{"quest claim", true, 179, ints(3, 4), map[string]any{"quest_id": int32(3), "reward_argument": int32(4)}},
		{"login rejection", false, 1, []byte{2}, map[string]any{"status": int8(2)}},
		{"system message", false, 3, join(ints(2), utf("Example")), map[string]any{"kind": int32(2), "message": "Example"}},
		{"server ping", false, 4, ints(2), map[string]any{"uninterpreted_value": int32(2)}},
		{"daily award", false, 13, join([]byte{1}, ints(0)), map[string]any{"available": true, "kind": int32(0)}},
		{"pet equipment", false, 68, join([]byte{33}, ints(1, 2, 3, 4, 5, 6)), map[string]any{"subcommand": uint8(33), "weapon_exp": int32(1), "weapon_level": int32(2), "clothes_exp": int32(3), "clothes_level": int32(4), "hat_exp": int32(5), "hat_level": int32(6)}},
		{"small bugle", false, 71, join(ints(12), utf("Example"), utf("Olá")), map[string]any{"sender_id": int32(12), "sender": "Example", "message": "Olá"}},
		{"big bugle", false, 72, join(ints(3, 12), utf("Example"), utf("Olá")), map[string]any{"big_bugle_type": int32(3), "sender_id": int32(12), "sender": "Example", "message": "Olá"}},
		{"edict versions", false, 75, ints(2, 4, 8), map[string]any{"versions": []int32{4, 8}}},
		{"vip", false, 92, join([]byte{1}, ints(2, 3), date, date, ints(4), []byte{0}), map[string]any{"vip_type": int8(1), "vip_level": int32(2), "vip_exp": int32(3), "vip_expire_day": dateText, "last_date": dateText, "vip_next_level_days_needed": int32(4), "can_take_vip_reward": false}},
		{"invite friends", false, 107, join(ints(5), utf("Example"), ints(2), date, ints(3, 4, 5, 6, 7)), map[string]any{"subcommand": int32(5), "invite_number": "Example", "success_count": int32(2), "last_fb_time": dateText, "reward_count_1": int32(3), "reward_count_2": int32(4), "reward_count_3": int32(5), "reward_count_4": int32(6), "server_id": int32(7)}},
		{"guild invitation", false, 128, join([]byte{4}, ints(1, 2), utf("Invitee"), ints(3), utf("Inviter"), ints(4), utf("Guild")), map[string]any{"subcommand": uint8(4), "invitation_id": int32(1), "uninterpreted_id_1": int32(2), "uninterpreted_name": "Invitee", "uninterpreted_id_2": int32(3), "inviter_name": "Inviter", "consortia_id": int32(4), "consortia_name": "Guild"}},
		{"invitation deletion", false, 129, join([]byte{13}, ints(7), []byte{1}, utf("Example")), map[string]any{"subcommand": uint8(13), "invitation_id": int32(7), "success": true, "message": "Example"}},
		{"catch beast", false, 145, []byte{32, 1}, map[string]any{"subcommand": uint8(32), "event": "CATCHBEAST_BEGIN", "is_begin": true}},
		{"ddplay", false, 145, join([]byte{74, 0}, date, date, ints(7, 8)), map[string]any{"subcommand": uint8(74), "event": "DDPLAY_BEGIN", "open": false, "begin_date": dateText, "end_date": dateText, "money": int32(7), "exchange_fold": int32(8)}},
		{"shop limits", false, 168, ints(1, 2, 3, 4, 1, 5, 6, 7), map[string]any{"shop_limits": []map[string]any{{"goods_id": int32(2), "limit_count": int32(3)}}, "consortia_id": int32(4), "consortia_shop_limits": []map[string]any{{"goods_id": int32(5), "limit_count": int32(6)}}, "uninterpreted_value": int32(7)}},
		{"quest finished", false, 179, ints(3), map[string]any{"quest_id": int32(3)}},
		{"gemstone spirits", false, 209, join([]byte{1, 1}, ints(1, 2, 3), utf("Example"), ints(4)), map[string]any{"subcommand": uint8(1), "update_flag": true, "spirits": []map[string]any{{"player_id": int32(2), "spirit_id": int32(3), "value": "Example", "equip_place": int32(4)}}}},
		{"gifts", false, 218, ints(1, 2, 1, 3, 4), map[string]any{"player_id": int32(1), "charm_gp": int32(2), "gifts": []map[string]any{{"template_id": int32(3), "amount": int32(4)}}}},
		{"enthrall", false, 227, join([]byte{1}, ints(3), []byte{1, 0}), map[string]any{"enabled": true, "interface_id": int32(3), "approved": true, "minor": false}},
		{"marriage listing", false, 239, join(ints(1), []byte{1}, ints(2), []byte{0}, utf("Example")), map[string]any{"marry_info_id": int32(1), "present": true, "player_id": int32(2), "publish_equipment": false, "introduction": "Example"}},
		{"removed marriage listing", false, 239, join(ints(1), []byte{0}), map[string]any{"marry_info_id": int32(1), "present": false}},
		{"first recharge", false, 259, join([]byte{1}, ints(1, 2, 3, 4, 5, 6, 1, 7, 8)), map[string]any{"recharged": true, "first_award_got": int32(1), "week_award_got": int32(2), "month_award_got": int32(3), "first_condition": int32(4), "week_condition": int32(5), "month_condition": int32(6), "rewards": []map[string]any{{"id": int32(7), "value": int32(8)}}}},
		{"rescue", false, 277, []byte{1, 0}, map[string]any{"subcommand": uint8(1), "is_begin": false}},
		{"dress models", false, 333, join([]byte{1}, ints(1, 2, 1, 3, 4)), map[string]any{"subcommand": uint8(1), "models": []map[string]any{{"model_id": int32(2), "items": []map[string]any{{"item_id": int32(3), "template_id": int32(4)}}}}}},
		{"selected dress", false, 333, join([]byte{2}, ints(7)), map[string]any{"subcommand": uint8(2), "current_model": int32(7)}},
		{"closed treasure", false, 343, []byte{1, 1, 0}, map[string]any{"subcommand": uint8(1), "notify": true, "activity_open": false}},
		{"open treasure", false, 343, join([]byte{1, 0, 1}, date, date, ints(7)), map[string]any{"subcommand": uint8(1), "notify": false, "activity_open": true, "start_date": dateText, "end_date": dateText, "need_money": int32(7)}},
		{"avatar collections", false, 402, join([]byte{5}, ints(1, 2, 3, 2, 4, 5), date), map[string]any{"subcommand": uint8(5), "collections": []map[string]any{{"id": int32(2), "category": int32(3), "active_items": []int32{4, 5}, "end_time": dateText}}}},
		{"daily checkin", false, 443, join([]byte{1}, ints(2, 1, 0, 2, 1)), map[string]any{"show": true, "days": []map[string]any{{"day": int32(1), "state": int32(0)}, {"day": int32(2), "state": int32(1)}}}},
		{"battle pass", false, 448, join([]byte{1}, ints(2, 3, 4, 5, 6, 7), utf("Example"), ints(8, 9, 10, 1, 11, 12)), map[string]any{"is_premium": true, "current_level": int32(2), "current_exp": int32(3), "premium_expire_days": int32(4), "exp_buff_bonus_pct": int32(5), "exp_buff_remaining_minutes": int32(6), "season_id": int32(7), "season_end_date": "Example", "total_levels": int32(8), "exp_per_level": int32(9), "featured_slot": int32(10), "collected": []map[string]any{{"level": int32(11), "track_type": int32(12)}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			direction := capture.ServerToClient
			if tt.client {
				direction = capture.ClientToServer
			}
			original := bytes.Clone(tt.body)
			got := NewDecoder().Decode(direction, framing.Frame{Header: framing.Header{Code: tt.code}, Body: tt.body})
			if got.Status != "decoded" || got.Err != nil || got.Consumed != len(tt.body) || len(got.Remaining) != 0 || !reflect.DeepEqual(got.Fields, tt.want) {
				t.Fatalf("result = %#v; want fields %#v", got, tt.want)
			}
			if !bytes.Equal(original, tt.body) {
				t.Fatal("input mutated")
			}
			for n := 0; n < len(tt.body); n++ {
				got = NewDecoder().Decode(direction, framing.Frame{Header: framing.Header{Code: tt.code}, Body: tt.body[:n]})
				if got.Status != "invalid" {
					t.Fatalf("prefix %d status = %s", n, got.Status)
				}
			}
		})
	}
}

func TestDailyActivityAndQuests(t *testing.T) {
	date := []byte{7, 234, 10, 14, 12, 34, 56}
	tests := []struct {
		name string
		code int16
		body []byte
		want map[string]any
	}{
		{"daily activity", 155, join([]byte{1}, ints(1, 2, 3, 1, 4, 5, 1, 6, 7, 8, 1, 9), []byte{1, 0}, utf("Example")), map[string]any{
			"subcommand": uint8(1), "activities": []map[string]any{{"id": int32(2), "completed_count": int32(3)}}, "buttons": []map[string]any{{"id": int32(4), "state": int32(5)}}, "sessions": []map[string]any{{"id": int32(6), "value": int32(7)}}, "activity_value": int32(8), "find_back": []map[string]any{{"id": int32(9), "flag_1": true, "flag_2": false}}, "speed_activities": "Example",
		}},
		{"quest", 178, join(ints(1, 123), []byte{1}, ints(1, 2, 3, 4), date, ints(5, 6), []byte{0}), map[string]any{"quests": []map[string]any{{"quest_id": int32(123), "achieved": true, "condition_1": int32(1), "condition_2": int32(2), "condition_3": int32(3), "condition_4": int32(4), "complete_date": "2026-10-14T12:34:56", "repeat_left": int32(5), "quality": int32(6), "exists": false}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: tt.code}, Body: tt.body})
			if got.Status != "decoded" || !reflect.DeepEqual(got.Fields, tt.want) {
				t.Fatalf("result = %#v", got)
			}
			for n := 0; n < len(tt.body); n++ {
				got = NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: tt.code}, Body: tt.body[:n]})
				if got.Status != "invalid" {
					t.Fatalf("prefix %d = %#v", n, got)
				}
			}
		})
	}
}
