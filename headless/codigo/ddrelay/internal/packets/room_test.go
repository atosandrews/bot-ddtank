package packets

import (
	"bytes"
	"reflect"
	"testing"

	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
)

func TestRoomBodies(t *testing.T) {
	tests := []struct {
		name   string
		client bool
		body   []byte
		want   map[string]any
	}{
		{"create request", true, join(ints(0), []byte{2, 3}, utf("Room"), utf("private-password")), map[string]any{"subcommand": int32(0), "room_type": int8(2), "time_type": int8(3), "name": "Room", "password": "[redacted]"}},
		{"login by id", true, join(ints(1), []byte{1}, ints(2, 3)), map[string]any{"subcommand": int32(1), "login_flag": true, "room_type": int32(2), "room_id": int32(3)}},
		{"login search", true, join(ints(1), []byte{0}, ints(2, -1, 4), utf("private-password")), map[string]any{"subcommand": int32(1), "login_flag": false, "room_type": int32(2), "room_id": int32(-1), "argument": int32(4), "password": "[redacted]"}},
		{"setup request", true, join(ints(2, 3), []byte{4}, utf("private-password"), utf("Room"), []byte{5, 6}, ints(7), []byte{1, 0}), map[string]any{"subcommand": int32(2), "map_id": int32(3), "room_type": int8(4), "password": "[redacted]", "name": "Room", "time_type": int8(5), "difficulty": int8(6), "level_limits": int32(7), "setup_flag_1": true, "setup_flag_2": false}},
		{"exit request", true, ints(5), map[string]any{"subcommand": int32(5)}},
		{"start request", true, ints(7), map[string]any{"subcommand": int32(7)}},
		{"list request", true, ints(9, 1, -2), map[string]any{"subcommand": int32(9), "list_type": int32(1), "filter": int32(-2)}},
		{"filtered list request", true, ints(9, 2, -2, 3, 4), map[string]any{"subcommand": int32(9), "list_type": int32(2), "filter": int32(-2), "argument_1": int32(3), "argument_2": int32(4)}},
		{"cancel request", true, ints(11), map[string]any{"subcommand": int32(11)}},
		{"state request", true, join(ints(15), []byte{2}), map[string]any{"subcommand": int32(15), "state": int8(2)}},
		{"created room", false, join([]byte{0}, ints(1), []byte{2, 3, 4, 5, 6, 7, 1}, ints(8), []byte{0}, utf("Room"), []byte{9}, ints(10), []byte{1, 0}), map[string]any{"subcommand": uint8(0), "room_id": int32(1), "room_type": int8(2), "difficulty": int8(3), "time_type": int8(4), "player_count": int8(5), "viewer_count": int8(6), "place_count": int8(7), "locked": true, "map_id": int32(8), "started": false, "name": "Room", "game_mode": int8(9), "level_limits": int32(10), "cross_zone": true, "within_league_time": false}},
		{"login result", false, []byte{1, 0}, map[string]any{"subcommand": uint8(1), "success": false}},
		{"setup changed", false, join([]byte{2, 1}, utf("Picture"), ints(3), []byte{4}, utf("private-password"), utf("Room"), []byte{5, 6}, ints(7), []byte{0}), map[string]any{"subcommand": uint8(2), "open_boss": true, "picture": "Picture", "map_id": int32(3), "room_type": int8(4), "password": "[redacted]", "name": "Room", "time_type": int8(5), "difficulty": int8(6), "level_limits": int32(7), "cross_zone": false}},
		{"self player prefix", false, join([]byte{4, 1, 2, 3, 0}, ints(4, 5, 6, 7, 8, 9)), map[string]any{"subcommand": uint8(4), "header_player_id": int32(123), "room_flag": true, "place": int8(2), "team": int8(3), "first_in": false, "grade": int32(4), "offer": int32(5), "hide": int32(6), "repute": int32(7), "uninterpreted_value": int32(8), "zone_id": int32(9), "layout": "common_prefix"}},
		{"removed player", false, join([]byte{5}, ints(7)), map[string]any{"subcommand": uint8(5), "player_id": int32(123), "zone_id": int32(7)}},
		{"places", false, join([]byte{10}, ints(0, 1, 2, 3, 4, 5, 6, 7, 8, 9)), map[string]any{"subcommand": uint8(10), "places": []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}},
		{"wait cancelled", false, []byte{11}, map[string]any{"subcommand": uint8(11)}},
		{"game style", false, []byte{12, 2}, map[string]any{"subcommand": uint8(12), "game_style": int8(2)}},
		{"waiting", false, []byte{13}, map[string]any{"subcommand": uint8(13)}},
		{"player states", false, []byte{15, 0, 1, 2, 3, 4, 5, 6, 7}, map[string]any{"subcommand": uint8(15), "player_states": []int8{0, 1, 2, 3, 4, 5, 6, 7}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			direction := capture.ServerToClient
			if tt.client {
				direction = capture.ClientToServer
			}
			frame := framing.Frame{Header: framing.Header{Code: 94, ClientID: 123}, Body: tt.body}
			got := NewDecoder().Decode(direction, frame)
			if got.Status != "decoded" || got.Consumed != len(tt.body) || !reflect.DeepEqual(got.Fields, tt.want) {
				t.Fatalf("result = %#v; want %#v", got, tt.want)
			}
			for n := 0; n < len(tt.body); n++ {
				frame.Body = tt.body[:n]
				got = NewDecoder().Decode(direction, frame)
				if got.Status != "invalid" {
					t.Fatalf("prefix %d = %#v", n, got)
				}
			}
		})
	}
	body := []byte{15, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 94}, Body: body})
	if got.Status != "partial" || !bytes.Equal(got.Remaining, []byte{8, 9}) {
		t.Fatalf("uninterpreted state tail lost: %#v", got)
	}
}

func TestRoomFullPlayer(t *testing.T) {
	prefix := join([]byte{4, 1, 2, 3, 0}, ints(4, 5, 6, 7, 8, 9))
	body := join(prefix, ints(10), utf("Example"), []byte{1}, ints(11), []byte{1}, utf("Style"), utf("Color"), utf("Skin"), ints(12, 13, 14), utf("Guild"), ints(15, 16, 17, 18, 19, 20), []byte{1}, ints(21), utf("Spouse"), utf("private-login"), ints(22, 1, 23, 24, 25), utf("Master"), ints(26), utf("Honor"), []byte{1}, ints(27, 1, 28, 29, 30), utf("Pet"), ints(31, 32, 1, 33, 34))
	got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 94}, Body: body})
	if got.Status != "decoded" || got.Consumed != len(body) {
		t.Fatalf("result = %#v", got)
	}
	for name, value := range map[string]any{"layout": "full_player", "player_id": int32(10), "spouse_id": int32(21), "login_name": "[redacted]", "fight_power": int64(4294967319), "daily_league_last_score": int32(27)} {
		if !reflect.DeepEqual(got.Fields[name], value) {
			t.Fatalf("%s = %#v; want %#v", name, got.Fields[name], value)
		}
	}
	wantPets := []map[string]any{{"place": int32(28), "template_id": int32(29), "id": int32(30), "name": "Pet", "user_id": int32(31), "level": int32(32), "skills": []map[string]any{{"slot": int32(33), "id": int32(34)}}}}
	if !reflect.DeepEqual(got.Fields["pets"], wantPets) {
		t.Fatalf("pets = %#v", got.Fields["pets"])
	}
	for n := 0; n < len(body); n++ {
		got = NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 94}, Body: body[:n]})
		if n == len(prefix) {
			if got.Status != "decoded" || got.Fields["layout"] != "common_prefix" {
				t.Fatalf("prefix variant = %#v", got)
			}
		} else if got.Status != "invalid" {
			t.Fatalf("prefix %d = %#v", n, got)
		}
	}
}
