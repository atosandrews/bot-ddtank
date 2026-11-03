package projection

import (
	"testing"

	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/value"
)

func TestWeaponSurvivesStageLoadAndDetachedSnapshots(t *testing.T) {
	session := game.NewSession(1, false)
	projector := NewProjector(session)
	rows := []map[string]any{{
		"living_id": int32(0),
		"player_id": int32(5),
		"nickname":  "Player",
		"team":      int32(1),
		"max_hp":    int32(100),
		"weapon_id": int32(4),
	}}

	for _, fields := range []map[string]any{{"subcommand": uint8(101), "players": rows}, {"subcommand": uint8(103), "map_id": int32(3)}} {
		if _, err := projector.Apply("ServerToClient", Header{Code: 91}, Decoded{Status: "decoded", Fields: fields}); err != nil {
			t.Fatal(err)
		}
	}

	actor := session.Snapshot().Fight.Actors[0]
	if actor.WeaponID == nil || *actor.WeaponID != 4 {
		t.Fatalf("actor = %+v", actor)
	}

	*actor.WeaponID = 99
	if got := *session.Snapshot().Fight.Actors[0].WeaponID; got != 4 {
		t.Fatalf("weapon changed to %d", got)
	}
}

func TestTurnResetsChangedProjectile(t *testing.T) {
	session := game.NewSession(1, false)
	projector := NewProjector(session)
	changed := map[string]any{"subcommand": uint8(20), "ball_id": int32(2)}
	if _, err := projector.Apply("ServerToClient", Header{Code: 91, Parameter1: 0}, Decoded{Status: "decoded", Fields: changed}); err != nil {
		t.Fatal(err)
	}

	turn := map[string]any{
		"subcommand":         uint8(6),
		"turn_number":        int32(2),
		"turn_time":          int32(20),
		"wind_display_flag":  true,
		"wind_display_bytes": []int8{1, 2, 3},
		"livings":            []map[string]any{},
	}
	if _, err := projector.Apply("ServerToClient", Header{Code: 91, Parameter1: 0}, Decoded{Status: "decoded", Fields: turn}); err != nil {
		t.Fatal(err)
	}

	if ball := session.Snapshot().Fight.Actors[0].BallID; ball != nil {
		t.Fatalf("old projectile retained: %d", *ball)
	}
}

func TestClientMovementExpiresPositionUntilServerUpdate(t *testing.T) {
	session := game.NewSession(1, false)
	state := session.Snapshot()
	state.Player.ID = value.Of(int32(9))
	state.Fight.Actors[0] = fight.Actor{ID: 0, PlayerID: value.Of(int32(9)), Position: &fight.Position{X: 10, Y: 20}}
	if err := session.Commit(state); err != nil {
		t.Fatal(err)
	}

	fields := map[string]any{"subcommand": uint8(9), "move_type": int8(0), "x": int32(30), "y": int32(40)}
	event, err := NewProjector(session).Apply("ClientToServer", Header{Code: 91}, Decoded{Status: "decoded", Fields: fields})
	if err != nil {
		t.Fatal(err)
	}

	actor := session.Snapshot().Fight.Actors[0]
	if event != "position_unconfirmed" || actor.Position != nil || actor.Destination == nil || *actor.Destination != (fight.Position{X: 30, Y: 40}) {
		t.Fatalf("event = %s, actor = %+v", event, actor)
	}
}

func TestClientStationaryWalkReportKeepsConfirmedPosition(t *testing.T) {
	session := game.NewSession(1, false)
	state := session.Snapshot()
	state.Player.ID = value.Of(int32(9))
	state.Fight.Actors[0] = fight.Actor{ID: 0, PlayerID: value.Of(int32(9)), Position: &fight.Position{X: 10, Y: 20}, Energy: value.Of(float64(240))}
	if err := session.Commit(state); err != nil {
		t.Fatal(err)
	}

	fields := map[string]any{"subcommand": uint8(9), "move_type": int8(0), "x": int32(10), "y": int32(20)}
	event, err := NewProjector(session).Apply("ClientToServer", Header{Code: 91}, Decoded{Status: "decoded", Fields: fields})
	if err != nil {
		t.Fatal(err)
	}

	actor := session.Snapshot().Fight.Actors[0]
	if event != "" || actor.Position == nil || *actor.Position != (fight.Position{X: 10, Y: 20}) || actor.Energy == nil || *actor.Energy != 240 {
		t.Fatalf("event = %s, actor = %+v", event, actor)
	}
}
