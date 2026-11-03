package packets

import (
	"reflect"
	"testing"

	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
)

func TestSocialBodies(t *testing.T) {
	tests := []struct {
		name      string
		direction capture.Direction
		code      int16
		body      []byte
		want      map[string]any
	}{
		{"add friend", capture.ClientToServer, 160, join([]byte{160}, utf("Friend"), ints(0), []byte{0, 1}), map[string]any{
			"subcommand": uint8(160),
			"nickname":   "Friend",
			"relation":   int32(0),
			"flag_1":     false,
			"flag_2":     true,
		}},
		{"invite", capture.ClientToServer, 70, ints(42), map[string]any{"player_id": int32(42)}},
		{"friend rejected", capture.ServerToClient, 160, []byte{160, 0}, map[string]any{"subcommand": uint8(160), "success": false}},
		{"friend notification", capture.ServerToClient, 160, join([]byte{166}, ints(7), utf("Friend"), []byte{1}), map[string]any{
			"subcommand": uint8(166),
			"argument":   int32(7),
			"nickname":   "Friend",
			"same_city":  true,
		}},
		{"invitation received", capture.ServerToClient, 70, join(ints(7, 8, 9), []byte{4, 3, 2, 1}, utf("Friend"), []byte{1}, ints(5), utf("Room"), utf("secret"), ints(2)), map[string]any{
			"player_id":      int32(7),
			"room_id":        int32(8),
			"map_id":         int32(9),
			"second_type":    int8(4),
			"game_mode":      int8(3),
			"difficulty":     int8(2),
			"level_limits":   int8(1),
			"nickname":       "Friend",
			"vip":            true,
			"vip_level":      int32(5),
			"room_name":      "Room",
			"password":       "[redacted]",
			"barrier_number": int32(2),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decode := func(body []byte) Result {
				return NewDecoder().Decode(tt.direction, framing.Frame{Header: framing.Header{Code: tt.code}, Body: body})
			}
			got := decode(tt.body)
			if got.Status != "decoded" || got.Err != nil || got.Consumed != len(tt.body) || !reflect.DeepEqual(got.Fields, tt.want) {
				t.Fatalf("result=%+v fields=%#v want=%#v", got, got.Fields, tt.want)
			}

			for n := 0; n < len(tt.body); n++ {
				if got := decode(tt.body[:n]); got.Status != "invalid" {
					t.Fatalf("prefix %d status=%s", n, got.Status)
				}
			}

			if got := decode(append(append([]byte{}, tt.body...), 99)); got.Status != "partial" || !reflect.DeepEqual(got.Remaining, []byte{99}) {
				t.Fatalf("tail=%+v", got)
			}
		})
	}
}

func TestFriendRecord(t *testing.T) {
	body := join([]byte{160, 1}, ints(7), utf("Friend"), []byte{2}, ints(3), []byte{1}, utf("style"), utf("colors"), utf("skin"), ints(1, 20, 0), utf("Guild"), ints(10, 1, 9, 4, 5, 0), utf("private-login"), ints(6, 1, 2, 3, 4), utf("Pupil"), ints(5), utf("Master"), ints(6), utf("Honor"), []byte{0})
	want := map[string]any{
		"subcommand":           uint8(160),
		"success":              true,
		"player_id":            int32(7),
		"nickname":             "Friend",
		"vip_type":             int8(2),
		"vip_level":            int32(3),
		"sex":                  true,
		"style":                "style",
		"colors":               "colors",
		"skin":                 "skin",
		"state":                int32(1),
		"grade":                int32(20),
		"hide":                 int32(0),
		"consortia_name":       "Guild",
		"total_count":          int32(10),
		"escape_count":         int32(1),
		"win_count":            int32(9),
		"offer":                int32(4),
		"repute":               int32(5),
		"relation":             int32(0),
		"login_name":           "[redacted]",
		"nimbus":               int32(6),
		"fight_power":          int64(4294967298),
		"apprenticeship_state": int32(3),
		"master_id":            int32(4),
		"apprentices":          "Pupil",
		"graduates_count":      int32(5),
		"master_honor":         "Master",
		"achievement_points":   int32(6),
		"honor":                "Honor",
		"married":              false,
	}

	for n := 2; n <= len(body); n++ {
		got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 160}, Body: body[:n]})
		if n < len(body) {
			if got.Status != "invalid" {
				t.Fatalf("prefix %d status=%s", n, got.Status)
			}

			continue
		}

		if got.Status != "decoded" || !reflect.DeepEqual(got.Fields, want) {
			t.Fatalf("result=%+v fields=%#v", got, got.Fields)
		}
	}
}
