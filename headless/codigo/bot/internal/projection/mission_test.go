package projection

import (
	"reflect"
	"testing"

	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/value"
)

func TestMissionLoadClearsPreviousStageActorsAndTurnState(t *testing.T) {
	session := game.NewSession(1, true)
	snapshot := session.Snapshot()
	snapshot.Fight = fight.State{
		Generation: 3, TurnRevision: 8, Phase: "mission_ended",
		MapID: value.Of(int32(100)), MissionID: value.Of(int32(202)),
		Turn: &fight.Turn{ActorID: 0, Number: 7},
		Wind: fight.Wind{
			Tenths:    value.Of(int32(-12)),
			Current:   true,
			Display:   &[3]int8{1, 0, 2},
			Direction: value.Of(int32(-1)),
			Source:    value.Of("display"),
			Pictures:  map[int8]fight.WindPicture{8: {SHA256: windZeroHash, Glyph: "0"}},
		},
		Actors: map[int32]fight.Actor{
			0: {
				ID:        0,
				PlayerID:  value.Of(int32(1001)),
				Name:      value.Of("Player"),
				Team:      value.Of(int32(1)),
				Position:  &fight.Position{X: 20, Y: 30},
				HP:        value.Of(int32(500)),
				PendingHP: value.Of(int32(400)),
				Alive:     value.Of(true),
				BallID:    value.Of(int32(7)),
			},
			1: {
				ID:       1,
				PlayerID: value.Of(int32(1002)),
				Name:     value.Of("Partner"),
				Team:     value.Of(int32(1)),
				HP:       value.Of(int32(600)),
			},
			9: {ID: 9, HP: value.Of(int32(100)), Team: value.Of(int32(2))},
		},
	}
	if err := session.Commit(snapshot); err != nil {
		t.Fatal(err)
	}

	name, err := NewProjector(session).Apply("ServerToClient", Header{Code: 91}, Decoded{
		Status: "decoded", Fields: map[string]any{"subcommand": uint8(103), "map_id": int32(101)},
	})
	if err != nil {
		t.Fatal(err)
	}

	expected := fight.State{
		Cards:      fight.Cards{Revision: 1},
		Generation: 3, TurnRevision: 9, Phase: "loading",
		MapID: value.Of(int32(101)), MissionID: value.Of(int32(202)),
		Actors: map[int32]fight.Actor{
			0: {
				ID:       0,
				PlayerID: value.Of(int32(1001)),
				Name:     value.Of("Player"),
				Team:     value.Of(int32(1)),
			},
			1: {
				ID:       1,
				PlayerID: value.Of(int32(1002)),
				Name:     value.Of("Partner"),
				Team:     value.Of(int32(1)),
			},
		},
	}
	if got := session.Snapshot().Fight; name != "fight_loading" || !reflect.DeepEqual(got, expected) {
		t.Fatalf("event = %q, state = %#v, want %#v", name, got, expected)
	}
}
