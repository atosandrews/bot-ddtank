package projection

import (
	"fmt"
	"reflect"
	"testing"

	"ddbot/bot/internal/game"
)

func TestSocialProjection(t *testing.T) {
	session := game.NewSession(1, true)
	projector := NewProjector(session)
	apply := func(code int32, fields map[string]any, want string) {
		t.Helper()
		name, err := projector.Apply("ServerToClient", Header{Code: code}, Decoded{Status: "decoded", Fields: fields})
		if err != nil || name != want {
			t.Fatalf("event=%q err=%v", name, err)
		}
	}
	apply(94, map[string]any{"subcommand": uint8(1), "success": false}, "room_join_rejected")
	if result := session.Snapshot().Room.LastJoinSuccess; result == nil || *result {
		t.Fatalf("join result=%v", result)
	}

	apply(94, map[string]any{"subcommand": uint8(1), "success": true}, "room_join_accepted")
	if result := session.Snapshot().Room.LastJoinSuccess; result == nil || !*result {
		t.Fatalf("join result=%v", result)
	}

	apply(160, map[string]any{
		"subcommand": uint8(160),
		"success":    true,
		"player_id":  int32(7),
		"nickname":   "Friend",
		"relation":   int32(0),
		"grade":      int32(20),
		"state":      int32(1),
	}, "friend_added")
	got := session.Snapshot().Social
	if got.Friends[7].Name != "Friend" || got.LastResult == nil || !got.LastResult.Success || *got.LastResult.PlayerID != 7 {
		t.Fatalf("social=%+v", got)
	}

	apply(160, map[string]any{"subcommand": uint8(160), "success": false}, "friend_rejected")
	got = session.Snapshot().Social
	if got.LastResult.Success || got.LastResult.PlayerID != nil || len(got.Friends) != 1 {
		t.Fatalf("rejected=%+v", got)
	}

	apply(160, map[string]any{
		"subcommand": uint8(166),
		"argument":   int32(8),
		"nickname":   "Other",
		"same_city":  true,
	}, "friend_notification")
	apply(70, map[string]any{
		"player_id":      int32(9),
		"room_id":        int32(10),
		"map_id":         int32(2),
		"second_type":    int8(4),
		"game_mode":      int8(3),
		"difficulty":     int8(0),
		"level_limits":   int8(0),
		"nickname":       "Host",
		"room_name":      "Room",
		"vip":            false,
		"vip_level":      int32(0),
		"barrier_number": int32(0),
	}, "room_invited")
	snapshot := session.Snapshot()
	if len(snapshot.Social.Invitations) != 1 || snapshot.Social.Invitations[0].RoomID != 10 || snapshot.Social.Notification.Argument != 8 {
		t.Fatalf("social=%+v", snapshot.Social)
	}

	delete(snapshot.Social.Friends, 7)
	snapshot.Social.Invitations[0].RoomID = 99
	if session.Snapshot().Social.Invitations[0].RoomID != 10 || len(session.Snapshot().Social.Friends) != 1 {
		t.Fatal("snapshot aliases state")
	}

	if other := game.NewSession(2, true).Snapshot().Social; len(other.Friends) != 0 || len(other.Invitations) != 0 || other.LastResult != nil {
		t.Fatalf("foreign state=%+v", other)
	}
}

func TestCardProjectionLifecycle(t *testing.T) {
	session := game.NewSession(1, true)
	p := NewProjector(session)
	apply := func(direction string, code int32, fields map[string]any, want string) {
		t.Helper()
		name, err := p.Apply(direction, Header{Code: code, ClientID: 7, Parameter1: 0}, Decoded{Status: "decoded", Fields: fields})
		if err != nil || name != want {
			t.Fatalf("event=%q err=%v", name, err)
		}
	}
	apply("ServerToClient", 91, map[string]any{
		"subcommand":       uint8(112),
		"take_card_type":   int32(2),
		"has_next_mission": false,
		"can_enter_final":  false,
		"players": []map[string]any{{
			"player_id":       int32(7),
			"level":           int32(20),
			"treatment":       int32(1),
			"gained_gp":       int32(10),
			"winner":          true,
			"card_count":      int32(2),
			"result_argument": int32(0),
			"has_level_again": false,
			"has_card_get":    true,
		}},
	}, "mission_ended")
	state := session.Snapshot().Fight.Cards
	if !state.Known || state.Type != 2 || state.Players[7].CardCount != 2 || len(state.Awards) != 0 {
		t.Fatalf("cards=%+v", state)
	}

	apply("ClientToServer", 91, map[string]any{"subcommand": uint8(114), "card_index": int8(3)}, "card_requested")
	apply("ServerToClient", 3, map[string]any{"kind": int32(1), "message": "Extra card unavailable"}, "server_message")
	state = session.Snapshot().Fight.Cards
	if len(state.Awards) != 0 || state.Requested[3] != "extra" {
		t.Fatalf("message fabricated reward=%+v", state)
	}

	if message := session.Snapshot().Message; message == nil || message.Text != "Extra card unavailable" {
		t.Fatalf("message=%+v", message)
	}

	apply("ServerToClient", 91, map[string]any{
		"subcommand":      uint8(98),
		"living_id":       int32(0),
		"player_id":       int32(7),
		"system_selected": false,
		"card_index":      int8(3),
		"template_id":     int32(100),
		"count":           int32(2),
		"vip":             true,
	}, "card_awarded")
	state = session.Snapshot().Fight.Cards
	if len(state.Requested) != 0 || state.Awards[3].TemplateID != 100 || state.Awards[3].Count != 2 || !*state.Awards[3].VIP {
		t.Fatalf("award=%+v", state)
	}

	apply("ServerToClient", 91, map[string]any{"subcommand": uint8(89), "cards": []map[string]any{{"card_index": int8(4), "template_id": int32(200), "count": int32(5)}}}, "cards_revealed")
	state = session.Snapshot().Fight.Cards
	if !state.Closed || len(state.Awards) != 1 || state.Revealed[4].TemplateID != 200 {
		t.Fatalf("reveal=%+v", state)
	}

	*state.Awards[3].VIP = false
	if !*session.Snapshot().Fight.Cards.Awards[3].VIP {
		t.Fatal("award pointer aliases state")
	}

	apply("ServerToClient", 91, map[string]any{"subcommand": uint8(103), "map_id": int32(1121)}, "fight_loading")
	next := session.Snapshot().Fight.Cards
	if next.Known || len(next.Awards) != 0 || len(next.Revealed) != 0 || len(next.Requested) != 0 || next.Revision <= state.Revision {
		t.Fatalf("stage state=%+v", next)
	}

	before := session.Snapshot()
	_, err := p.Apply("ServerToClient", Header{Code: 91}, Decoded{Status: "decoded", Fields: map[string]any{"subcommand": uint8(98), "card_index": int8(2)}})
	if err == nil || !reflect.DeepEqual(before, session.Snapshot()) {
		t.Fatal("invalid projection committed")
	}
}

func TestIncompleteCardEventsDisableSelection(t *testing.T) {
	for _, sub := range []uint8{98, 89, 112, 103, 101} {
		t.Run(fmt.Sprint(sub), func(t *testing.T) {
			s := game.NewSession(1, true)
			snapshot := s.Snapshot()
			snapshot.Fight.Cards.Known = true
			snapshot.Fight.Cards.Type = 2
			snapshot.Fight.Cards.Revision = 5
			if err := s.Commit(snapshot); err != nil {
				t.Fatal(err)
			}

			event, err := NewProjector(s).Apply("ServerToClient", Header{Code: 91}, Decoded{Status: "partial", Fields: map[string]any{"subcommand": sub}})
			if err != nil || event != "cards_unknown" {
				t.Fatalf("event=%s error=%v", event, err)
			}

			if cards := s.Snapshot().Fight.Cards; cards.Known || cards.Revision != 6 {
				t.Fatalf("cards=%+v", cards)
			}
		})
	}
}

func TestCardEmptyResultAndSessionIsolation(t *testing.T) {
	for _, kind := range []int32{1, 2} {
		for _, system := range []bool{false, true} {
			t.Run(fmt.Sprintf("board_%d_system_%t", kind, system), func(t *testing.T) {
				s := game.NewSession(1, true)
				next := s.Snapshot()
				next.Fight.Cards.Known = true
				next.Fight.Cards.Type = kind
				if err := s.Commit(next); err != nil {
					t.Fatal(err)
				}

				name, err := NewProjector(s).Apply("ServerToClient", Header{Code: 91}, Decoded{Status: "decoded", Fields: map[string]any{
					"subcommand":      uint8(98),
					"card_index":      int8(50),
					"system_selected": system,
				}})
				if err != nil || name != "card_selection_empty" {
					t.Fatalf("event=%s error=%v", name, err)
				}

				if cards := s.Snapshot().Fight.Cards; cards.SystemSelection != (kind == 2 && system) || len(cards.Awards) != 0 {
					t.Fatalf("cards=%+v", cards)
				}

				if cards := game.NewSession(2, true).Snapshot().Fight.Cards; cards.Known || cards.SystemSelection || len(cards.Awards) != 0 {
					t.Fatalf("foreign cards=%+v", cards)
				}
			})
		}
	}
}
