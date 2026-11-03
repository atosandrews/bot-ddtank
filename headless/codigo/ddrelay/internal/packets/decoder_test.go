package packets

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
)

func TestDecodeBodies(t *testing.T) {
	tests := []struct {
		name      string
		direction capture.Direction
		code      int16
		body      []byte
		fields    map[string]any
	}{
		{"client ping", capture.ClientToServer, 4, []byte{7, 234, 10, 14, 15, 34, 11, 0}, map[string]any{"client_time": "2026-10-14T15:34:11", "flag": false}},
		{"network", capture.ServerToClient, 6, ints(179), map[string]any{"player_id": int32(123), "web_speed": int32(179)}},
		{"remove player", capture.ServerToClient, 21, nil, map[string]any{"player_id": int32(123)}},
		{"client exit", capture.ClientToServer, 21, nil, map[string]any{}},
		{"client chat", capture.ClientToServer, 19, join([]byte{5, 1}, utf("Example"), utf("Olá\nworld")), map[string]any{"channel": int8(5), "team": true, "sender": "Example", "message": "Olá\nworld"}},
		{"server chat", capture.ServerToClient, 19, join(ints(2), []byte{5, 0}, utf("Example"), utf("Olá")), map[string]any{"sender_id": int32(123), "zone_id": int32(2), "channel": int8(5), "team": false, "sender": "Example", "message": "Olá"}},
		{"tutorial request", capture.ClientToServer, 15, join([]byte{1}, ints(7), []byte{1}), map[string]any{"kind": uint8(1), "step": int32(7), "flag": true}},
		{"tutorial response", capture.ServerToClient, 15, join(ints(3), []byte{1, 0, 255}), map[string]any{"tutorial_progress": []int8{1, 0, -1}}},
		{"quest request", capture.ClientToServer, 176, ints(2, 100, 200), map[string]any{"quest_ids": []int32{100, 200}}},
		{"quest check", capture.ClientToServer, 181, ints(1, 2, 3), map[string]any{"argument_1": int32(1), "argument_2": int32(2), "argument_3": int32(3)}},
		{"invitation delete", capture.ClientToServer, 129, ints(13, 7), map[string]any{"subcommand": int32(13), "invitation_id": int32(7)}},
		{"friend state", capture.ClientToServer, 160, join([]byte{165}, ints(1)), map[string]any{"subcommand": uint8(165), "state": int32(1)}},
		{"equipment lookup", capture.ClientToServer, 74, join([]byte{0}, utf("Example")), map[string]any{"by_id": false, "nickname": "Example"}},
		{"marriage request", capture.ClientToServer, 236, join([]byte{1}, utf("example"), ints(7)), map[string]any{"flag": true, "text": "example", "argument": int32(7)}},
		{"payment info request", capture.ClientToServer, 266, ints(2), map[string]any{"subcommand": int32(2)}},
		{"notice", capture.ServerToClient, 10, join(ints(2), utf("Example"), []byte{1}, ints(0, 100, 200)), map[string]any{"notice_type": int32(2), "message": "Example", "item_links": []map[string]any{{"index": int32(0), "template_id": int32(100), "item_id": int32(200)}}}},
		{"empty ranked request", capture.ClientToServer, 700, nil, map[string]any{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := bytes.Clone(tt.body)
			got := NewDecoder().Decode(tt.direction, framing.Frame{Header: framing.Header{Code: tt.code, ClientID: 123}, Body: tt.body})
			if got.Status != "decoded" || got.Err != nil || got.Consumed != len(tt.body) || len(got.Remaining) != 0 || !reflect.DeepEqual(got.Fields, tt.fields) {
				t.Fatalf("result = %#v; fields = %#v; want %#v", got, got.Fields, tt.fields)
			}
			if !bytes.Equal(original, tt.body) {
				t.Fatal("input mutated")
			}
		})
	}
}

func TestStructuredLists(t *testing.T) {
	player := join(ints(20), []byte{1}, utf("Example"), []byte{2}, ints(3), utf("Guild"), ints(4, 5, 6, 7, 8, 9), []byte{1}, ints(42), utf("Spouse"), utf("private-login"), ints(1, 2, 3))
	room := join(ints(7), []byte{0, 3, 1, 0, 2, 4, 1}, ints(100), []byte{0}, utf("Example room"), []byte{1, 2}, ints(30))
	activity := join(ints(2, 1), utf("activity-a"), ints(1, 10, 20, 1), utf("gift-a"), ints(2, 3, 4))
	rankedBody := join(ints(1), []byte{2, 3}, ints(4, 5, 6), []byte{0, 7, 0, 8}, ints(0, 9, 0, 10, 0, 11), []byte{12, 13, 1}, ints(14, 15, 16, 1), []byte{2}, ints(100, 3))
	tests := []struct {
		name       string
		code       int16
		body       []byte
		compressed bool
		want       map[string]any
	}{
		{"scene player", 18, player, false, map[string]any{"player": map[string]any{"player_id": int32(123), "grade": int32(20), "sex": true, "nickname": "Example", "vip_type": int8(2), "vip_level": int32(3), "consortia_name": "Guild", "offer": int32(4), "win_count": int32(5), "total_count": int32(6), "escape_count": int32(7), "consortia_id": int32(8), "repute": int32(9), "married": true, "spouse_id": int32(42), "spouse_name": "Spouse", "login_name": "[redacted]", "fight_power": int64(4294967298), "apprenticeship_state": int32(3)}}},
		{"room list", 94, join([]byte{9}, ints(50, 1), room), false, map[string]any{"subcommand": uint8(9), "room_total": int32(50), "room_count": 1, "rooms": []map[string]any{{"id": int32(7), "type": int8(0), "time_type": int8(3), "player_count": int8(1), "viewer_count": int8(0), "max_viewers": int8(2), "place_count": int8(4), "locked": true, "map_id": int32(100), "playing": false, "name": "Example room", "game_mode": int8(1), "hard_level": int8(2), "level_limits": int32(30)}}}},
		{"compressed activities", 405, activity, true, map[string]any{"kind": int32(2), "activity_count": 1, "activities": []map[string]any{{"id": "activity-a", "statuses": []map[string]any{{"id": int32(10), "value": int32(20)}}, "gifts": []map[string]any{{"id": "gift-a", "times": int32(2), "all_gift_get_times": int32(3), "remaining_times": int32(4)}}}}}},
		{"ranked", 700, rankedBody, false, map[string]any{"season_id": int32(1), "elo_tier": uint8(2), "elo_division": uint8(3), "ranked_pdl": int32(4), "total_wins": int32(5), "total_matches": int32(6), "win_streak": int16(7), "max_streak": int16(8), "icon_unlock": int64(9), "title_unlock": int64(10), "elo_reward_bits": int64(11), "selected_icon": uint8(12), "selected_title": uint8(13), "season_active": true, "season_remaining_seconds": int32(14), "division_floor": int32(15), "next_division_target": int32(16), "rewards": []map[string]any{{"elo_tier": uint8(2), "template_id": int32(100), "count": int32(3)}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := tt.body
			if tt.compressed {
				wire = compressed(t, wire)
			}
			got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: tt.code, ClientID: 123}, Body: wire})
			if got.Status != "decoded" || got.Err != nil || got.Consumed != len(tt.body) || !reflect.DeepEqual(got.Fields, tt.want) {
				t.Fatalf("result = %#v; fields = %#v; want %#v", got, got.Fields, tt.want)
			}
			if tt.compressed && got.Compression != "zlib" {
				t.Fatal("compression missing")
			}
			for n := 0; n < len(tt.body); n++ {
				truncated := tt.body[:n]
				if tt.compressed {
					truncated = compressed(t, truncated)
				}
				result := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: tt.code}, Body: truncated})
				if result.Status != "invalid" {
					t.Fatalf("prefix %d status = %s", n, result.Status)
				}
			}
		})
	}
	bulk := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 1800}, Body: join(ints(1, 123), player)})
	if bulk.Status != "decoded" || !reflect.DeepEqual(bulk.Fields["players"], []map[string]any{tests[0].want["player"].(map[string]any)}) {
		t.Fatalf("bulk = %#v", bulk)
	}
}

func TestFailuresAndUnknownBodies(t *testing.T) {
	tests := []struct {
		name      string
		code      int16
		body      []byte
		status    string
		err       error
		remaining int
	}{
		{"unknown code", 32000, []byte{1, 2, 3}, "unknown", ErrUnsupported, 3},
		{"unknown room subcommand", 94, []byte{255, 1, 2}, "partial", ErrUnsupported, 2},
		{"extra bytes", 21, []byte{1, 2}, "partial", nil, 2},
		{"short integer", 6, []byte{1}, "invalid", io.ErrUnexpectedEOF, 1},
		{"negative count", 1800, ints(-1), "invalid", ErrInvalid, 0},
		{"oversized count", 1800, ints(1000000), "invalid", ErrInvalid, 0},
		{"invalid UTF", 19, join(ints(1), []byte{5, 0, 0, 1, 255}), "invalid", ErrInvalid, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: tt.code}, Body: tt.body})
			if got.Status != tt.status || !errors.Is(got.Err, tt.err) || len(got.Remaining) != tt.remaining {
				t.Fatalf("result = %#v", got)
			}
			if !bytes.Equal(got.Remaining, tt.body[got.Consumed:]) {
				t.Fatal("unread bytes not preserved")
			}
		})
	}
}

func TestCompressionBoundaries(t *testing.T) {
	valid := compressed(t, ints(0))
	corrupt := bytes.Clone(valid)
	corrupt[len(corrupt)-1] ^= 1
	tests := []struct {
		name string
		body []byte
	}{
		{"truncated", valid[:len(valid)-1]},
		{"checksum", corrupt},
		{"trailing", append(bytes.Clone(valid), 0)},
		{"expansion limit", compressed(t, []byte(strings.Repeat("x", maxInflatedBody+1)))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 405}, Body: tt.body})
			if got.Status != "invalid" || got.Err == nil || !bytes.Equal(got.Remaining, tt.body) {
				t.Fatalf("result = %#v", got)
			}
		})
	}
}

func TestLongEncoding(t *testing.T) {
	tests := []struct {
		high, low int32
		want      int64
	}{{1, 2, 4294967298}, {-1, 2, -4294967298}, {0, -1, 4294967295}}
	for _, tt := range tests {
		r := &reader{data: ints(tt.high, tt.low)}
		if got := r.long("value"); got != tt.want || r.err != nil {
			t.Fatalf("long = %d, err %v, want %d", got, r.err, tt.want)
		}
	}
}

func FuzzDecode(f *testing.F) {
	f.Add(int16(94), []byte{9, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add(int16(1800), ints(0))
	f.Add(int16(405), []byte{0x78, 0xda})
	f.Fuzz(func(t *testing.T, code int16, data []byte) {
		if len(data) > 65535 {
			return
		}
		original := bytes.Clone(data)
		got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: code}, Body: data})
		if got.Consumed < 0 || got.Consumed > got.Total || got.Total-got.Consumed != len(got.Remaining) {
			t.Fatal("invalid accounting")
		}
		if !bytes.Equal(original, data) {
			t.Fatal("input mutated")
		}
	})
}

func ints(values ...int32) []byte {
	var data []byte
	for _, value := range values {
		data = binary.BigEndian.AppendUint32(data, uint32(value))
	}
	return data
}

func utf(value string) []byte {
	return append(binary.BigEndian.AppendUint16(nil, uint16(len(value))), []byte(value)...)
}

func join(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

func compressed(t *testing.T, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	w := zlib.NewWriter(&out)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestProgressPackets(t *testing.T) {
	date := []byte{7, 234, 10, 14, 15, 34, 11}
	tests := []struct {
		name string
		code int16
		body []byte
		want map[string]any
	}{
		{"achievement records", 229, ints(2, 10, 20, 30, 40), map[string]any{"records": []map[string]any{{"record_id": int32(10), "total": int32(20)}, {"record_id": int32(30), "total": int32(40)}}}},
		{"achievement dates", 231, ints(1, 42, 2026, 10, 14), map[string]any{"achievements": []map[string]any{{"id": int32(42), "year": int32(2026), "month": int32(10), "day": int32(14)}}}},
		{"novice activity", 258, join(ints(0, 1, 2, 3), date, date), map[string]any{"subcommand": int32(0), "activity_type": int32(1), "conditions": int32(2), "award_got": int32(3), "start_date": "2026-10-14T15:34:11", "end_date": "2026-10-14T15:34:11"}},
		{"limited charge open", 342, join(ints(0), []byte{1}), map[string]any{"subcommand": int32(0), "open": true}},
		{"limited charge records", 342, join(ints(1, 1, 123, 45, 67), []byte{1}), map[string]any{"subcommand": int32(1), "users": []map[string]any{{"user_id": int32(123), "activity_id": int32(45), "activity_amount": int32(67), "finished": true}}}},
		{"buff records", 186, join(ints(1, 42), []byte{1}, date, ints(1, 2, 3, 4)), map[string]any{"buffs": []map[string]any{{"type": int32(42), "flag": true, "date": "2026-10-14T15:34:11", "argument_1": int32(1), "argument_2": int32(2), "argument_3": int32(3), "argument_4": int32(4)}}}},
		{"buff update", 185, join(ints(1, 42), []byte{1}, date, ints(1, 2, 3, 4)), map[string]any{"buffs": []map[string]any{{"type": int32(42), "flag": true, "date": "2026-10-14T15:34:11", "argument_1": int32(1), "argument_2": int32(2), "argument_3": int32(3), "argument_4": int32(4)}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: tt.code}, Body: tt.body})
			if got.Status != "decoded" || got.Consumed != len(tt.body) || !reflect.DeepEqual(got.Fields, tt.want) {
				t.Fatalf("result = %#v; fields = %#v; want %#v", got, got.Fields, tt.want)
			}
		})
	}
}

func TestPlayerInfo(t *testing.T) {
	date := []byte{7, 234, 10, 14, 15, 34, 11}
	body := join(
		ints(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16),
		utf("style"), utf("colors"), utf("skin"), []byte{1}, ints(17), utf("guild"),
		ints(18, 19, 20, 21), utf("pve"), utf("mission"), ints(0, 22, 23, 24), utf("master"),
		ints(25), utf("master honor"), ints(26), utf("honor"), []byte{1}, ints(27), date,
		ints(28, 29), date, ints(30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43), date,
		ints(1, 44), []byte{2}, ints(45, 46), []byte{1},
	)
	got := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 67, ClientID: 123}, Body: body})
	if got.Status != "decoded" || got.Consumed != len(body) {
		t.Fatalf("result = %#v", got)
	}
	want := map[string]any{"player_id": int32(123), "gp": int64(4294967298), "hide": int32(16), "style": "style", "consortia_name": "guild", "nimbus": int32(21), "fight_power": int64(22), "master_id": int32(24), "last_spa_date": "2026-10-14T15:34:11", "use_offer": int32(30), "training_count": int32(43), "dungeon_flags": []map[string]any{{"id": int32(44), "flag": int8(2)}}, "evolution_exp": int32(46), "dd_pass": true}
	for key, value := range want {
		if !reflect.DeepEqual(got.Fields[key], value) {
			t.Fatalf("field %s = %#v, want %#v", key, got.Fields[key], value)
		}
	}
	for n := 0; n < len(body); n++ {
		truncated := NewDecoder().Decode(capture.ServerToClient, framing.Frame{Header: framing.Header{Code: 67}, Body: body[:n]})
		if truncated.Status != "invalid" {
			t.Fatalf("prefix %d status = %s", n, truncated.Status)
		}
	}
}
