package packets

import (
	"testing"

	"ddbot/relay/internal/capture"
)

func TestFightCardResults(t *testing.T) {
	for _, tt := range []struct {
		name string
		body []byte
		want map[string]any
	}{
		{"small", join([]byte{98, 0, 3}, ints(900, 2)), map[string]any{"subcommand": uint8(98), "event": "TAKE_CARD", "living_id": int32(77), "player_id": int32(123), "system_selected": false, "card_index": int8(3), "template_id": int32(900), "count": int32(2)}},
		{"large vip", join([]byte{98, 1, 4}, ints(901, 1), []byte{1}), map[string]any{"subcommand": uint8(98), "event": "TAKE_CARD", "living_id": int32(77), "player_id": int32(123), "system_selected": true, "card_index": int8(4), "template_id": int32(901), "count": int32(1), "vip": true}},
		{"sentinel", []byte{98, 1, 50}, map[string]any{"subcommand": uint8(98), "event": "TAKE_CARD", "living_id": int32(77), "player_id": int32(123), "system_selected": true, "card_index": int8(50)}},
		{"reveal", join([]byte{89}, ints(2), []byte{0}, ints(902, 3), []byte{8}, ints(903, 1)), map[string]any{"subcommand": uint8(89), "event": "SHOW_CARDS", "cards": []map[string]any{{"card_index": int8(0), "template_id": int32(902), "count": int32(3)}, {"card_index": int8(8), "template_id": int32(903), "count": int32(1)}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertFight(t, decodeFight(capture.ServerToClient, tt.body), tt.body, tt.want)
			for size := 0; size < len(tt.body); size++ {
				got := decodeFight(capture.ServerToClient, tt.body[:size])
				want := "invalid"
				if tt.name == "large vip" && size == len(tt.body)-1 {
					want = "decoded"
				}
				if got.Status != want {
					t.Fatalf("prefix %d status = %q, want %q", size, got.Status, want)
				}
			}
		})
	}
}

func TestFightCardRevealRejectsBadCountsAndRetainsExtraBytes(t *testing.T) {
	for _, count := range []int32{-1, maxEntries + 1, 2} {
		body := join([]byte{89}, ints(count), []byte{0}, ints(900, 1))
		if got := decodeFight(capture.ServerToClient, body); got.Status != "invalid" {
			t.Fatalf("count %d = %#v", count, got)
		}
	}

	body := join([]byte{98, 0, 3}, ints(900, 2), []byte{0, 55})
	got := decodeFight(capture.ServerToClient, body)
	if got.Status != "partial" || got.Consumed != len(body)-1 || len(got.Remaining) != 1 || got.Remaining[0] != 55 {
		t.Fatalf("extra tail = %#v", got)
	}
}
