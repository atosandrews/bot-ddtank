package projection

import (
	"testing"

	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/value"
)

func TestShootCountTracksLocalPlayerAndResetsOnTurn(t *testing.T) {
	for _, test := range []struct {
		name        string
		roomType    int32
		headerActor int32
		want        int32
	}{
		{"ordinary room uses local actor", 4, 99, 2},
		{"activity room checks actor", 21, 99, 1},
		{"activity local actor", 21, 0, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			session := game.NewSession(1, false)
			state := session.Snapshot()
			state.Player.ID = value.Of(int32(7))
			state.Fight.RoomType = value.Of(test.roomType)
			state.Fight.Actors[0] = fight.Actor{ID: 0, PlayerID: value.Of(int32(7)), ShootCount: value.Of(int32(1))}
			if err := session.Commit(state); err != nil {
				t.Fatal(err)
			}
			projector := NewProjector(session)
			event, err := projector.Apply("ServerToClient", Header{Code: 91, Parameter1: test.headerActor}, Decoded{Status: "decoded", Fields: map[string]any{"subcommand": uint8(46), "shoot_count": int8(2)}})
			if err != nil || event != "shoot_count_changed" {
				t.Fatalf("event=%s error=%v", event, err)
			}
			actor := session.Snapshot().Fight.Actors[0]
			if actor.ShootCount == nil || *actor.ShootCount != test.want {
				t.Fatal("wrong shot allowance")
			}
			*actor.ShootCount = 9
			if *session.Snapshot().Fight.Actors[0].ShootCount != test.want {
				t.Fatal("snapshot mutated owner")
			}
			if _, err := projector.Apply("ServerToClient", Header{Code: 91}, Decoded{Status: "partial", Fields: map[string]any{"subcommand": uint8(46)}}); err != nil {
				t.Fatal(err)
			}
			if session.Snapshot().Fight.Actors[0].ShootCount != nil {
				t.Fatal("incomplete update retained extra shot allowance")
			}
			turn := map[string]any{"subcommand": uint8(6), "turn_number": int32(2), "turn_time": int32(30), "wind_display_flag": true, "wind_display_bytes": []int8{1, 0, 1}, "livings": []map[string]any{{"living_id": int32(0), "x": int32(100), "y": int32(200), "hp": int32(50), "max_hp": int32(50), "shoot_count": int32(1)}}}
			if _, err := projector.Apply("ServerToClient", Header{Code: 91, Parameter1: 0}, Decoded{Status: "decoded", Fields: turn}); err != nil {
				t.Fatal(err)
			}
			if *session.Snapshot().Fight.Actors[0].ShootCount != 1 {
				t.Fatal("extra shot survived next turn")
			}
			if _, err := projector.Apply("ServerToClient", Header{Code: 91}, Decoded{Status: "decoded", Fields: map[string]any{"subcommand": uint8(103), "map_id": int32(1121)}}); err != nil {
				t.Fatal(err)
			}
			if session.Snapshot().Fight.Actors[0].ShootCount != nil {
				t.Fatal("extra shot survived mission load")
			}
		})
	}
}
