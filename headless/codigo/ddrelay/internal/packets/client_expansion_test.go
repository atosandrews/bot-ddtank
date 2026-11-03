package packets

import (
	"bytes"
	"reflect"
	"testing"

	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
)

func TestClientTracedLayouts(t *testing.T) {
	tests := []struct {
		name   string
		server bool
		code   int16
		body   []byte
		want   map[string]any
	}{
		{"blast", false, 91, join([]byte{3}, ints(1, 2, 3, 4)), map[string]any{"subcommand": uint8(3), "event": "BLAST", "argument_1": int32(1), "argument_2": int32(2), "argument_3": int32(3), "argument_4": int32(4)}},
		{"stop moving", false, 91, join([]byte{10}, ints(100, -20), []byte{1}), map[string]any{"subcommand": uint8(10), "event": "MOVESTOP", "x": int32(100), "y": int32(-20), "move_flag": true}},
		{"skip turn", false, 91, []byte{12, 7}, map[string]any{"subcommand": uint8(12), "event": "SKIPNEXT", "argument": int8(7)}},
		{"change ball request", false, 91, join([]byte{19}, ints(1, 2, 3, 4, 5)), map[string]any{"subcommand": uint8(19), "event": "CHANGEBALL", "argument_1": int32(1), "argument_2": int32(2), "argument_3": int32(3), "argument_4": int32(4), "argument_5": int32(5)}},
		{"use prop", false, 91, join([]byte{32, 1}, ints(2, 100)), map[string]any{"subcommand": uint8(32), "event": "PROP", "bag_type": int8(1), "place": int32(2), "template_id": int32(100)}},
		{"take card", false, 91, []byte{98, 4}, map[string]any{"subcommand": uint8(98), "event": "TAKE_CARD", "card_index": int8(4)}},
		{"paid card", false, 91, []byte{114, 3}, map[string]any{"subcommand": uint8(114), "event": "PAYMENT_TAKE_CARD", "card_index": int8(3)}},
		{"boss card", false, 91, []byte{130, 2}, map[string]any{"subcommand": uint8(130), "event": "BOSS_TAKE_CARD", "card_index": int8(2)}},
		{"prepare mission", false, 91, []byte{116, 1}, map[string]any{"subcommand": uint8(116), "event": "GAME_MISSION_PREPARE", "ready": true}},
		{"retry mission", false, 91, join([]byte{119}, ints(2), []byte{1, 0}), map[string]any{"subcommand": uint8(119), "event": "GAME_MISSION_TRY_AGAIN", "argument": int32(2), "flag_1": true, "flag_2": false}},
		{"room kick", false, 94, join(ints(3), []byte{2}), map[string]any{"subcommand": int32(3), "place": int8(2)}},
		{"room team request", false, 94, join(ints(6), []byte{1}), map[string]any{"subcommand": int32(6), "team": int8(1)}},
		{"room place request", false, 94, join(ints(10), []byte{2}, ints(-1), []byte{0}, ints(3)), map[string]any{"subcommand": int32(10), "place": int8(2), "state": int32(-1), "place_flag": false, "argument": int32(3)}},
		{"room style request", false, 94, ints(12, 2), map[string]any{"subcommand": int32(12), "game_style": int32(2)}},
		{"single room", false, 94, ints(18, 3), map[string]any{"subcommand": int32(18), "argument": int32(3)}},
		{"delete prop outer packet", false, 75, ints(3), map[string]any{"argument": int32(3)}},
		{"mission start outer packet", false, 82, []byte{1}, map[string]any{"flag": true}},
		{"team change", true, 94, []byte{6, 2, 3}, map[string]any{"subcommand": uint8(6), "player_id": int32(123), "team": int8(2), "place": int8(3)}},
		{"room password prompt", true, 94, join([]byte{14}, ints(7)), map[string]any{"subcommand": uint8(14), "room_id": int32(7)}},
		{"health update", true, 91, join([]byte{11, 1}, ints(800, 200)), map[string]any{"subcommand": uint8(11), "event": "HEALTH", "living_id": int32(456), "change_type": int8(1), "hp": int32(800), "effect_amount": int32(200)}},
		{"show health", true, 91, join([]byte{80}, ints(100, -1)), map[string]any{"subcommand": uint8(80), "event": "LIVING_SHOW_BLOOD", "living_id": int32(100), "visibility_value": int32(-1), "visible": true}},
		{"hide health", true, 91, join([]byte{80}, ints(100, 0)), map[string]any{"subcommand": uint8(80), "event": "LIVING_SHOW_BLOOD", "living_id": int32(100), "visibility_value": int32(0), "visible": false}},
		{"mission ready update", true, 91, []byte{116, 0}, map[string]any{"subcommand": uint8(116), "event": "GAME_MISSION_PREPARE", "player_id": int32(123), "ready": false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			direction := capture.ClientToServer
			if tt.server {
				direction = capture.ServerToClient
			}
			frame := framing.Frame{Header: framing.Header{Code: tt.code, ClientID: 123, Parameter1: 456}, Body: tt.body}
			original := bytes.Clone(tt.body)
			got := NewDecoder().Decode(direction, frame)
			if got.Status != "decoded" || got.Err != nil || got.Consumed != len(tt.body) || !reflect.DeepEqual(got.Fields, tt.want) {
				t.Fatalf("result = %#v; want %#v", got, tt.want)
			}
			if !bytes.Equal(original, tt.body) {
				t.Fatal("input mutated")
			}
			for n := 0; n < len(tt.body); n++ {
				frame.Body = tt.body[:n]
				got = NewDecoder().Decode(direction, frame)
				if got.Status != "invalid" {
					t.Fatalf("prefix %d accepted: %#v", n, got)
				}
			}
			frame.Body = join(tt.body, []byte{0xaa, 0xbb})
			got = NewDecoder().Decode(direction, frame)
			if got.Status != "partial" || !bytes.Equal(got.Remaining, []byte{0xaa, 0xbb}) {
				t.Fatalf("unread tail lost: %#v", got)
			}
		})
	}
}

func TestDirectionAndPacketNamespace(t *testing.T) {
	for _, tt := range []struct {
		direction    capture.Direction
		code         int16
		body         []byte
		status, name string
	}{
		{capture.ClientToServer, 75, ints(2), "decoded", "PROP_DELETE"},
		{capture.ServerToClient, 75, ints(1, 2), "decoded", "EDICTUM_GET_SERVION"},
		{capture.ClientToServer, 82, []byte{1}, "decoded", "GAME_MISSION_START"},
		{capture.ClientToServer, 91, []byte{82, 1}, "partial", "GAME_CMD"},
		{capture.ClientToServer, 91, join([]byte{75}, ints(2)), "partial", "GAME_CMD"},
		{capture.ServerToClient, 167, make([]byte, 128), "unknown", "CHURCH_MOVIE_OVER"},
	} {
		got := NewDecoder().Decode(tt.direction, framing.Frame{Header: framing.Header{Code: tt.code}, Body: tt.body})
		if got.Status != tt.status || got.Name != tt.name {
			t.Fatalf("result = %#v", got)
		}
	}
}
