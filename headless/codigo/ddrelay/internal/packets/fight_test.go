package packets

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"reflect"
	"testing"

	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
)

func TestFightRequests(t *testing.T) {
	tests := []struct {
		name string
		sub  byte
		data []byte
		want map[string]any
	}{
		{"shot", 2, ints(100, 200, 900, -45), map[string]any{"x": int32(100), "y": int32(200), "force": int32(900), "angle": int32(-45)}},
		{"direction", 7, ints(-1), map[string]any{"direction": int32(-1)}},
		{"move", 9, join([]byte{1, 2}, ints(-10, 20), []byte{255, 1, 0, 42}), map[string]any{"moving": true, "move_type": int8(2), "x": int32(-10), "y": int32(20), "direction": int8(-1), "is_living": true, "turn_number": int16(42)}},
		{"stunt", 15, []byte{3}, map[string]any{"stunt_type": int8(3)}},
		{"load", 16, ints(100), map[string]any{"progress": int32(100)}},
		{"airplane", 40, nil, map[string]any{}},
		{"shoot begin", 96, []byte{1, 2}, map[string]any{"started": true, "shot_type": int8(2)}},
		{"shoot end", 96, []byte{0}, map[string]any{"started": false}},
		{"auto", 149, []byte{1}, map[string]any{"enabled": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := join([]byte{tt.sub}, tt.data)
			want := tt.want
			want["subcommand"] = uint8(tt.sub)
			want["event"] = fightNames[tt.sub]
			got := decodeFight(capture.ClientToServer, body)
			assertFight(t, got, body, want)
			assertFightTruncations(t, capture.ClientToServer, body, nil)
		})
	}
}

func TestFightServerEvents(t *testing.T) {
	tests := []struct {
		name string
		sub  byte
		data []byte
		want map[string]any
	}{
		{"round end", 4, nil, map[string]any{}},
		{"direction", 7, ints(-1), map[string]any{"living_id": int32(77), "direction": int32(-1)}},
		{"dander", 14, ints(50), map[string]any{"living_id": int32(77), "dander": int32(50)}},
		{"loading", 16, ints(100, 2, 3), map[string]any{"progress": int32(100), "zone_id": int32(2), "player_id": int32(3)}},
		{"ball", 20, join([]byte{1}, ints(4)), map[string]any{"living_id": int32(77), "special": true, "ball_id": int32(4)}},
		{"prop", 32, join([]byte{2}, ints(3, 4, 5), []byte{1}), map[string]any{"living_id": int32(77), "bag_type": int8(2), "place": int32(3), "template_id": int32(4), "target_id": int32(5), "show_message": true}},
		{"property", 41, join(utf("speed"), utf("2")), map[string]any{"living_id": int32(77), "property": "speed", "value": "2"}},
		{"move", 55, join(ints(1, 2, 3, 4, 5), utf("walk"), utf("stand")), map[string]any{"living_id": int32(77), "from_x": int32(1), "from_y": int32(2), "to_x": int32(3), "to_y": int32(4), "speed": int32(5), "action": "walk", "end_action": "stand"}},
		{"fall", 56, join(ints(10, 20, 30), utf("fall"), ints(40)), map[string]any{"living_id": int32(77), "x": int32(10), "y": int32(20), "speed": int32(30), "action": "fall", "fall_argument": int32(40)}},
		{"say", 59, join(utf("Olá!"), ints(2)), map[string]any{"living_id": int32(77), "text": "Olá!", "say_type": int32(2)}},
		{"shoot tag", 96, []byte{1, 2}, map[string]any{"player_id": int32(123), "started": true, "shot_type": int8(2)}},
		{"barrier", 104, ints(1, 2, 3, 4), map[string]any{"current_value_1": int32(1), "current_value_2": int32(2), "current_value_3": int32(3), "current_value_4": int32(4)}},
		{"clock", 131, ints(300), map[string]any{"lifetime": int32(300)}},
		{"auto self", 149, join(ints(-1), []byte{1}), map[string]any{"count": int32(-1), "self_enabled": true}},
		{"auto list", 149, join(ints(2, 3), []byte{1}, ints(4), []byte{0}), map[string]any{"count": int32(2), "players": []map[string]any{{"living_id": int32(3), "enabled": true}, {"living_id": int32(4), "enabled": false}}}},
		{"psychic", 252, ints(5, 6), map[string]any{"psychic": int32(5), "living_id": int32(6)}},
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
}

func TestFightProjectiles(t *testing.T) {
	projectile := join(ints(1, 2), []byte{1}, ints(3, 100, 200, -30, -40, 5), utf("spark"), ints(1500, 2000, 2), ints(6, 2, 7, 8, 9, 10), ints(11, 999, 12, 13, 14, 15))
	body := join([]byte{2}, ints(-25), []byte{1, 2, 3, 4}, ints(1), projectile, ints(1, 9, 50, 100, 20, 3))
	want := map[string]any{
		"subcommand": uint8(2), "event": "FIRE", "living_id": int32(77), "wind_tenths": int32(-25), "wind_display_flag": true, "wind_display_bytes": []int8{2, 3, 4},
		"projectiles": []map[string]any{{"number": int32(1), "shoot_count": int32(2), "creates_hole": true, "id": int32(3), "x": int32(100), "y": int32(200), "vx": int32(-30), "vy": int32(-40), "template_id": int32(5), "changed_particle": "spark", "damage_modifier_1_thousandths": int32(1500), "damage_modifier_2_thousandths": int32(2000), "actions": []map[string]any{
			{"time": int32(6), "type": int32(2), "name": "BOMB", "parameter_1": int32(7), "parameter_2": int32(8), "parameter_3": int32(9), "parameter_4": int32(10), "x": int32(7), "y": int32(8)},
			{"time": int32(11), "type": int32(999), "name": "UNKNOWN", "parameter_1": int32(12), "parameter_2": int32(13), "parameter_3": int32(14), "parameter_4": int32(15)},
		}}}, "pet_targets": []map[string]any{{"target_id": int32(9), "damage": int32(50), "hp": int32(100), "dander": int32(20)}}, "pet_attack_type": int32(3),
	}
	assertFight(t, decodeFight(capture.ServerToClient, body), body, want)
	assertFightTruncations(t, capture.ServerToClient, body, nil)
	multi := join([]byte{2}, ints(0), []byte{0, 0, 0, 0}, ints(2), projectile, projectile, ints(0, 0))
	got := decodeFight(capture.ServerToClient, multi)
	if got.Status != "decoded" || len(got.Fields["projectiles"].([]map[string]any)) != 2 {
		t.Fatalf("multi = %#v", got)
	}
}

func TestFightTurnAndSpawn(t *testing.T) {
	turn := join([]byte{6, 1, 2, 3, 4, 0}, ints(30, 1, 10, 20, 30, 40, 1, 50), []byte{1}, ints(60, 70, 80, 90, 100), []byte{1}, ints(110, 120, 130, 140, 150, 160, 170))
	want := map[string]any{"subcommand": uint8(6), "event": "TURN", "living_id": int32(77), "wind_display_flag": true, "wind_display_bytes": []int8{2, 3, 4}, "hidden": false, "turn_time": int32(30), "layout": "player", "boxes": []map[string]any{{"id": int32(10), "x": int32(20), "y": int32(30), "type": int32(40)}}, "livings": []map[string]any{{"living_id": int32(50), "alive": true, "x": int32(60), "y": int32(70), "power_ratio": int32(80), "max_hp": int32(90), "hp": int32(100), "no_hole": true, "max_energy": int32(110), "dander": int32(120), "pet_max_mp": int32(130), "pet_mp": int32(140), "shoot_count": int32(150), "fly_count": int32(160)}}, "turn_number": int32(170)}
	assertFight(t, decodeFight(capture.ServerToClient, turn), turn, want)
	assertFightTruncations(t, capture.ServerToClient, turn, map[int]bool{30: true})
	spawn := join([]byte{64, 3}, ints(4), utf("Enemy"), utf("model"), utf("born"), ints(10, 20, 30, 40, 2), []byte{255, 0, 1, 0}, ints(1), utf("stand"), utf("idle"), ints(2, 11, 12), []byte{1, 0, 1, 0}, ints(1), utf("speed"), utf("5"))
	want = map[string]any{"subcommand": uint8(64), "event": "ADD_LIVING", "living_type": int8(3), "living_id": int32(4), "name": "Enemy", "model": "model", "action": "born", "x": int32(10), "y": int32(20), "hp": int32(30), "max_hp": int32(40), "team": int32(2), "direction": int8(-1), "layer": int8(0), "spawn_flag_1": true, "spawn_flag_2": false, "action_mapping": []map[string]any{{"name": "stand", "value": "idle"}}, "buff_ids": []int32{11, 12}, "frozen": true, "hidden": false, "no_hole": true, "state_flag": false, "properties": []map[string]any{{"name": "speed", "value": "5"}}}
	assertFight(t, decodeFight(capture.ServerToClient, spawn), spawn, want)
	assertFightTruncations(t, capture.ServerToClient, spawn, nil)
}

func TestFightOpaqueAndUnsupported(t *testing.T) {
	image := []byte("synthetic opaque picture")
	body := join([]byte{241, 7}, image)
	want := map[string]any{"subcommand": uint8(241), "event": "WIND_PIC", "picture_id": int8(7), "picture_bytes": len(image), "picture_sha256": fmt.Sprintf("%x", sha256.Sum256(image)), "picture_glyph_error": "wind picture PNG header: png: invalid format: not a PNG file"}
	assertFight(t, decodeFight(capture.ServerToClient, body), body, want)
	tests := []struct {
		name   string
		body   []byte
		status string
	}{
		{"unknown", []byte{250, 1, 2}, "partial"},
		{"style variant", []byte{134, 2, 1, 2}, "partial"},
		{"truncated header", nil, "invalid"},
		{"extra data", []byte{4, 1}, "partial"},
		{"negative list", join([]byte{58}, utf("hit"), ints(-1)), "invalid"},
		{"oversized list", join([]byte{67}, ints(10001)), "invalid"},
		{"auto invalid count", join([]byte{149}, ints(-2)), "invalid"},
		{"auto short list", join([]byte{149}, ints(3, 4)), "invalid"},
		{"ambiguous turn", join([]byte{6, 0, 0, 0, 0, 0}, ints(10, 0)), "partial"},
		{"invalid utf", []byte{59, 0, 1, 255}, "invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeFight(capture.ServerToClient, tt.body)
			if got.Status != tt.status || got.Consumed+len(got.Remaining) != len(tt.body) {
				t.Fatalf("result = %#v", got)
			}
			if tt.status == "partial" && !bytes.Equal(got.Remaining, tt.body[got.Consumed:]) {
				t.Fatal("unknown bytes lost")
			}
		})
	}
}

func decodeFight(direction capture.Direction, body []byte) Result {
	return NewDecoder().Decode(direction, framing.Frame{Header: framing.Header{Code: 91, ClientID: 123, Parameter1: 77, Parameter2: 88}, Body: body})
}

func assertFight(t *testing.T, got Result, body []byte, want map[string]any) {
	t.Helper()
	if got.Name != "GAME_CMD" || got.Status != "decoded" || got.Err != nil || got.Consumed != len(body) || got.Total != len(body) || len(got.Remaining) != 0 || !reflect.DeepEqual(got.Fields, want) {
		t.Fatalf("result = %#v\nfields = %#v\nwant = %#v", got, got.Fields, want)
	}
}

func assertFightTruncations(t *testing.T, direction capture.Direction, body []byte, partial map[int]bool) {
	t.Helper()
	for n := 0; n < len(body); n++ {
		original := bytes.Clone(body[:n])
		got := decodeFight(direction, body[:n])
		want := "invalid"
		if partial[n] {
			want = "partial"
		}
		if got.Status != want {
			t.Fatalf("prefix %d status=%s; error=%v", n, got.Status, got.Err)
		}
		if !bytes.Equal(original, body[:n]) {
			t.Fatal("input mutated")
		}
	}
}

func FuzzFight(f *testing.F) {
	for _, sub := range []byte{2, 4, 6, 7, 9, 14, 15, 16, 20, 32, 40, 41, 55, 56, 58, 59, 64, 67, 89, 96, 98, 99, 101, 103, 104, 112, 113, 115, 131, 134, 149, 241, 252, 250} {
		f.Add(false, sub, make([]byte, 32))
		f.Add(true, sub, make([]byte, 16))
	}
	f.Fuzz(func(t *testing.T, client bool, sub byte, data []byte) {
		if len(data) > 65514 {
			return
		}
		direction := capture.ServerToClient
		if client {
			direction = capture.ClientToServer
		}
		body := join([]byte{sub}, data)
		original := bytes.Clone(body)
		got := decodeFight(direction, body)
		if got.Consumed < 0 || got.Consumed > len(body) || got.Total != len(body) || got.Consumed+len(got.Remaining) != len(body) {
			t.Fatal("bad byte accounting")
		}
		if !bytes.Equal(body, original) {
			t.Fatal("input mutated")
		}
		if got.Status == "decoded" && (got.Err != nil || len(got.Remaining) > 0) {
			t.Fatal("false decoded status")
		}
	})
}
