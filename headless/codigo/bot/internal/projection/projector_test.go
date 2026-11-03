package projection

import (
	"testing"

	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/value"
)

func TestProjectorAppliesServerPositionOnly(t *testing.T) {
	session := game.NewSession(1, false)
	projector := NewProjector(session)
	header := Header{Code: 91, Parameter1: 7}
	decoded := Decoded{Status: "decoded", Fields: map[string]any{"subcommand": uint8(56), "x": int32(10), "y": int32(20)}}
	name, err := projector.Apply("ServerToClient", header, decoded)
	if err != nil {
		t.Fatal(err)
	}

	actor := session.Snapshot().Fight.Actors[7]
	if name != "actor_moved" || actor.Position != nil || actor.Destination == nil || actor.Destination.X != 10 || actor.Destination.Y != 20 {
		t.Fatalf("name = %s; actor = %#v", name, actor)
	}

	before := session.Snapshot().Revision
	_, err = projector.Apply("ClientToServer", header, decoded)
	if err != nil {
		t.Fatal(err)
	}

	if session.Snapshot().Revision != before {
		t.Fatal("client request changed state")
	}
}

func TestInvalidPacketDoesNotChangeState(t *testing.T) {
	session := game.NewSession(1, false)
	_, err := NewProjector(session).Apply("ServerToClient", Header{Code: 91}, Decoded{Status: "invalid"})
	if err != nil {
		t.Fatal(err)
	}

	if session.Snapshot().Revision != 0 {
		t.Fatal("invalid packet changed state")
	}
}

func TestProjectionIsAtomicOnBadFields(t *testing.T) {
	session := game.NewSession(1, false)
	decoded := Decoded{Status: "decoded", Fields: map[string]any{"subcommand": uint8(64), "living_id": int32(9), "x": int32(5)}}
	_, err := NewProjector(session).Apply("ServerToClient", Header{Code: 91}, decoded)
	if err == nil {
		t.Fatal("accepted incomplete actor")
	}

	state := session.Snapshot()
	if state.Revision != 0 || len(state.Fight.Actors) != 0 {
		t.Fatalf("partial commit = %#v", state)
	}
}

func TestHealthUsesClientFlashBackSemantics(t *testing.T) {
	for _, tt := range []struct {
		name    string
		kind    int8
		initial *int32
		want    *int32
	}{
		{"replace", 0, value.Of(int32(100)), value.Of(int32(60))},
		{"flash back", 7, value.Of(int32(100)), value.Of(int32(85))},
		{"unknown prior health", 7, nil, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session := game.NewSession(1, false)
			state := session.Snapshot()
			state.Fight.Actors[2] = fight.Actor{ID: 2, HP: tt.initial}
			if err := session.Commit(state); err != nil {
				t.Fatal(err)
			}

			fields := map[string]any{
				"subcommand":    uint8(11),
				"change_type":   tt.kind,
				"hp":            int32(60),
				"effect_amount": int32(15),
			}
			if _, err := NewProjector(session).Apply("ServerToClient", Header{Code: 91, Parameter1: 2}, Decoded{Status: "decoded", Fields: fields}); err != nil {
				t.Fatal(err)
			}

			got := session.Snapshot().Fight.Actors[2].HP
			if (got == nil) != (tt.want == nil) || got != nil && *got != *tt.want {
				t.Fatalf("health = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewTurnExpiresNumericWindAndPendingHealth(t *testing.T) {
	session := game.NewSession(1, false)
	state := session.Snapshot()
	state.Fight.Wind = fight.Wind{Tenths: value.Of(int32(-15)), Current: true}
	state.Fight.Actors[5] = fight.Actor{ID: 5, PendingHP: value.Of(int32(10))}
	if err := session.Commit(state); err != nil {
		t.Fatal(err)
	}

	fields := map[string]any{
		"subcommand":         uint8(6),
		"turn_number":        int32(3),
		"turn_time":          int32(20),
		"wind_display_flag":  true,
		"wind_display_bytes": []int8{1, 2, 3},
		"livings": []map[string]any{{
			"living_id": int32(5),
			"x":         int32(30),
			"y":         int32(40),
			"hp":        int32(10),
			"max_hp":    int32(100),
		}},
	}
	if _, err := NewProjector(session).Apply("ServerToClient", Header{Code: 91, Parameter1: 5}, Decoded{Status: "decoded", Fields: fields}); err != nil {
		t.Fatal(err)
	}

	got := session.Snapshot().Fight
	if got.Wind.Current || got.Wind.Tenths != nil || got.Turn.Number != 3 || got.Actors[5].PendingHP != nil || *got.Actors[5].HP != 10 {
		t.Fatalf("fight state = %#v", got)
	}
}

func TestIncompleteTurnCannotRetainPreviousTurnAuthority(t *testing.T) {
	session := game.NewSession(1, false)
	state := session.Snapshot()
	state.Fight.Turn = &fight.Turn{ActorID: 1, Number: 2}
	state.Fight.Wind.Current = true
	if err := session.Commit(state); err != nil {
		t.Fatal(err)
	}

	decoded := Decoded{Status: "partial", Fields: map[string]any{"subcommand": uint8(6)}}
	if _, err := NewProjector(session).Apply("ServerToClient", Header{Code: 91}, decoded); err != nil {
		t.Fatal(err)
	}

	got := session.Snapshot().Fight
	if got.Turn != nil || got.Wind.Current || got.TurnRevision != 1 {
		t.Fatalf("old turn authority retained = %#v", got)
	}
}
