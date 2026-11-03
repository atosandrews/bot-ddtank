package packets

import (
	"bytes"
	"reflect"
	"testing"

	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
)

func TestLoginBody(t *testing.T) {
	date := []byte{7, 234, 10, 14, 12, 34, 56}
	body := join(
		[]byte{0}, ints(1, 2, 3, 4, 5, 6, 7, 1, 8, 9, 10, 11, 12, 13, 14, 15, 1, 16),
		ints(17, 18), utf("Master"), ints(19), utf("Honor"), date,
		[]byte{2}, ints(20, 21), date, date, ints(22), date, []byte{1},
		ints(23, 24), utf("Title"), ints(25), []byte{0}, utf("Style&Colors"), utf("Skin"),
		ints(26), utf("Guild"), ints(27, 28), utf("Duty"), ints(29), utf("Chairman"), ints(30, 31), []byte{1},
		utf("private-question-one"), utf("private-question-two"), ints(32), utf("private-login"),
		ints(33), utf("Permission"), utf("Mission"), ints(34, 35, 36, 37), date, date,
		ints(38, 39, 40, 41), []byte{1}, ints(42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52), date,
		[]byte{1}, ints(53, 54), date, ints(55, 56, 57, 58), []byte{0}, utf("private-instance"),
	)
	got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 1}, Body: body})
	want := map[string]any{
		"zone_id": int32(1), "armor": int32(7), "gp": int64(4294967304), "fight_power": int64(4294967312),
		"master_or_apprentices": "Master", "vip_type": int8(2), "vip_next_level_days_needed": int32(22),
		"style_and_colors": "Style&Colors", "consortia_riches": int32(31), "bag_password_set": true,
		"security_question_1": "[redacted]", "security_question_2": "[redacted]", "login_name": "[redacted]", "client_instance": "[redacted]",
		"security_attempts_left": int32(32), "box_time_needed": int32(37), "daily_league_last_score": int32(42),
		"speed_training_exp": int32(46), "training_count": int32(52), "old_player_valid_equipment": true,
		"last_lucky_number": int32(55), "accumulative_award_days": int32(58), "old_player": false,
	}
	if got.Status != "decoded" || got.Consumed != len(body) {
		t.Fatalf("result = %#v", got)
	}
	for name, value := range want {
		if !reflect.DeepEqual(got.Fields[name], value) {
			t.Fatalf("field %s = %#v; want %#v", name, got.Fields[name], value)
		}
	}
	for n := 0; n < len(body); n++ {
		got = NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 1}, Body: body[:n]})
		if got.Status != "invalid" {
			t.Fatalf("prefix %d = %#v", n, got)
		}
	}
}

func syntheticInventoryItem(equipment, gold bool) []byte {
	date := []byte{7, 234, 10, 14, 12, 34, 56}
	var prefix []byte
	if equipment {
		prefix = []byte{2}
	}
	prefix = join(prefix, ints(1, 2, 3, 4, 5, 6, 7, 8, 9, 10))
	if !equipment {
		prefix = join(prefix, ints(11))
	}
	body := join(prefix, []byte{1, 0}, date, ints(12), utf("Color"), utf("Skin"), []byte{1}, ints(13, 14, 15, 16, 17, 18), utf("Picture"), ints(19), date)
	if !equipment {
		body = join(body, ints(20))
	}
	body = join(body, []byte{21}, ints(22), []byte{23}, ints(24))
	if gold {
		body = join(body, []byte{1}, ints(25), date)
	} else {
		body = join(body, []byte{0})
	}
	return join(body, utf("Current"), utf("New"), date)
}

func TestInventoryVariants(t *testing.T) {
	tests := []struct {
		name            string
		equipment, gold bool
	}{
		{"inventory normal", false, false}, {"inventory gold", false, true},
		{"equipment normal", true, false}, {"equipment gold", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := syntheticInventoryItem(tt.equipment, tt.gold)
			r := &reader{data: body}
			fields := make(map[string]any)
			inventoryItem(r, fields, tt.equipment)
			want := map[string]any{
				"user_id": int32(1), "item_id": int32(2), "count": int32(3), "place": int32(4), "template_id": int32(5),
				"attack_compose": int32(6), "defend_compose": int32(7), "agility_compose": int32(8), "luck_compose": int32(9), "strengthen_level": int32(10),
				"bound": true, "judged": false, "begin_date": "2026-10-14T12:34:56", "valid_date": int32(12), "color": "Color", "skin": "Skin", "used": true,
				"hole_1": int32(13), "hole_2": int32(14), "hole_3": int32(15), "hole_4": int32(16), "hole_5": int32(17), "hole_6": int32(18), "picture": "Picture", "refinery_level": int32(19),
				"discolor_valid_date": "2026-10-14T12:34:56", "hole_5_level": int8(21), "hole_5_exp": int32(22), "hole_6_level": int8(23), "hole_6_exp": int32(24), "gold": tt.gold,
				"latent_energy_current": "Current", "latent_energy_new": "New", "latent_energy_end_time": "2026-10-14T12:34:56",
			}
			if tt.equipment {
				want["bag_type"] = int8(2)
			} else {
				want["strengthen_exp"] = int32(11)
				want["strengthen_times"] = int32(20)
			}
			if tt.gold {
				want["gold_valid_date"] = int32(25)
				want["gold_begin_time"] = "2026-10-14T12:34:56"
			}
			if r.err != nil || r.offset != len(body) || !reflect.DeepEqual(fields, want) {
				t.Fatalf("fields = %#v; reader = %#v", fields, r)
			}
			for n := 0; n < len(body); n++ {
				r = &reader{data: body[:n]}
				inventoryItem(r, make(map[string]any), tt.equipment)
				if r.err == nil {
					t.Fatalf("prefix %d accepted", n)
				}
			}
		})
	}
}

func TestCompressedInventory(t *testing.T) {
	plain := join(ints(2, 3, 4), []byte{1}, syntheticInventoryItem(false, true), ints(5), []byte{0}, ints(6), []byte{1}, syntheticInventoryItem(false, false))
	wire := compressed(t, plain)
	original := bytes.Clone(wire)
	got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 64}, Body: wire})
	if got.Status != "decoded" || got.Compression != "zlib" || got.Consumed != len(plain) || !bytes.Equal(original, wire) {
		t.Fatalf("result = %#v", got)
	}
	slots := got.Fields["slots"].([]map[string]any)
	if got.Fields["bag_type"] != int32(2) || len(slots) != 3 || !reflect.DeepEqual(slots[1], map[string]any{"place": int32(5), "present": false}) || slots[0]["item"].(map[string]any)["gold"] != true || slots[2]["item"].(map[string]any)["gold"] != false {
		t.Fatalf("slots = %#v", slots)
	}
	for n := 0; n < len(plain); n++ {
		got = NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 64}, Body: compressed(t, plain[:n])})
		if got.Status != "invalid" {
			t.Fatalf("prefix %d accepted", n)
		}
	}
}

func TestCompressedEquipment(t *testing.T) {
	date := []byte{7, 234, 10, 14, 12, 34, 56}
	plain := join(ints(1), utf("Example"), ints(2, 3, 4, 5), utf("Color"), utf("Skin"), ints(6, 1, 7, 8, 9, 10, 11, 12), []byte{1}, utf("Style"), ints(13), []byte{2}, ints(14), []byte{1}, ints(15, 16, 17, 18), utf("Guild"), ints(19, 20, 21), []byte{0}, ints(22), utf("Spouse"), utf("Duty"), ints(23, 1, 24, 25, 26), utf("Master"), ints(27), utf("Honor"), ints(28), utf("Title"), date, ints(29, 30, 31, 32, 33), []byte{1}, ints(34, 35, 2), syntheticInventoryItem(true, true), syntheticInventoryItem(true, false))
	got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 74}, Body: compressed(t, plain)})
	if got.Status != "decoded" || got.Compression != "zlib" || got.Consumed != len(plain) {
		t.Fatalf("result = %#v", got)
	}
	for name, value := range map[string]any{"player_id": int32(1), "nickname": "Example", "gp": int64(4294967303), "fight_power": int64(4294967320), "master_id": int32(26), "totem_id": int32(35)} {
		if !reflect.DeepEqual(got.Fields[name], value) {
			t.Fatalf("%s = %#v, want %#v", name, got.Fields[name], value)
		}
	}
	items := got.Fields["items"].([]map[string]any)
	if len(items) != 2 || items[0]["gold"] != true || items[1]["gold"] != false || items[1]["template_id"] != int32(5) {
		t.Fatalf("items = %#v", items)
	}
	for n := 0; n < len(plain); n++ {
		got = NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 74}, Body: compressed(t, plain[:n])})
		if got.Status != "invalid" {
			t.Fatalf("prefix %d accepted", n)
		}
	}
	tail := []byte{9, 8, 7, 6}
	got = NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 74}, Body: compressed(t, join(plain, tail))})
	if got.Status != "partial" || !bytes.Equal(got.Remaining, tail) {
		t.Fatalf("uninterpreted tail lost: %#v", got)
	}
}
