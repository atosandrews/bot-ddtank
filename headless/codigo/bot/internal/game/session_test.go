package game

import (
	"errors"
	"testing"

	"ddbot/bot/internal/game/action"
	"ddbot/bot/internal/game/fight"
)

func TestSessionCommitsDetachedState(t *testing.T) {
	session := NewSession(3, false)
	next := session.Snapshot()
	hp := int32(100)
	next.Fight.Actors[7] = fight.Actor{ID: 7, HP: &hp}
	if err := session.Commit(next); err != nil {
		t.Fatal(err)
	}

	hp = 2
	view := session.Snapshot()
	*view.Fight.Actors[7].HP = 5
	got := session.Snapshot()
	if got.Revision != 1 || *got.Fight.Actors[7].HP != 100 || got.Stream != 3 {
		t.Fatalf("snapshot = %#v", got)
	}
}

func TestReadOnlySessionRejectsCommands(t *testing.T) {
	_, err := NewSession(1, false).Request(action.Shoot{Force: 100, Angle: -45})
	var unavailable *action.UnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("error = %v", err)
	}
}

func TestGapInvalidatesStateAndQueuedCommands(t *testing.T) {
	session := NewSession(1, true)
	if _, err := session.Request(action.RoomStart{}); err != nil {
		t.Fatal(err)
	}

	session.Invalidate("capture gap")
	if session.Snapshot().Valid {
		t.Fatal("state remains valid")
	}

	_, err := session.Request(action.RoomStart{})
	var unavailable *action.UnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("error = %v", err)
	}

	if len(session.Pending()) != 0 {
		t.Fatal("queued commands survived gap")
	}
}

func TestCommandsAreScopedToOwnTurn(t *testing.T) {
	for _, tt := range []struct {
		name        string
		playerID    int32
		nextTurn    int32
		wantRequest bool
		wantValid   bool
	}{
		{"own turn", 42, 3, true, true},
		{"other player", 99, 3, false, false},
		{"turn changed", 42, 4, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session := NewSession(1, true)
			state := session.Snapshot()
			self := int32(42)
			state.Player.ID = &self
			state.Fight.Phase = "active"
			state.Fight.Generation = 7
			state.Fight.Turn = &fight.Turn{ActorID: 5, Number: 3}
			state.Fight.Actors[5] = fight.Actor{ID: 5, PlayerID: &tt.playerID}
			if err := session.Commit(state); err != nil {
				t.Fatal(err)
			}

			_, err := session.Request(action.Shoot{Force: 100, Angle: -45})
			if (err == nil) != tt.wantRequest {
				t.Fatalf("request = %v", err)
			}

			if !tt.wantRequest {
				return
			}

			pending := session.Pending()
			state = session.Snapshot()
			state.Fight.Turn.Number = tt.nextTurn
			if err := session.Commit(state); err != nil {
				t.Fatal(err)
			}

			if err := session.Validate(pending[0]); (err == nil) != tt.wantValid {
				t.Fatalf("validate = %v", err)
			}
		})
	}
}

func TestSessionRejectsStaleCommitAndFullQueue(t *testing.T) {
	session := NewSession(1, true)
	stale := session.Snapshot()
	if err := session.Commit(stale); err != nil {
		t.Fatal(err)
	}

	if err := session.Commit(stale); err == nil {
		t.Fatal("accepted stale snapshot")
	}

	for range MaxPendingCommands {
		if _, err := session.Request(action.RoomStart{}); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := session.Request(action.RoomStart{}); err == nil {
		t.Fatal("accepted overflowing queue")
	}

	if got := len(session.Pending()); got != MaxPendingCommands {
		t.Fatalf("pending = %d", got)
	}
}

func TestRepeatedTurnNumberDoesNotReviveOldRequest(t *testing.T) {
	session := NewSession(1, true)
	state := session.Snapshot()
	playerID := int32(42)
	state.Player.ID = &playerID
	state.Fight.Phase = "active"
	state.Fight.Turn = &fight.Turn{ActorID: 1, Number: 1}
	state.Fight.Actors[1] = fight.Actor{ID: 1, PlayerID: &playerID}
	if err := session.Commit(state); err != nil {
		t.Fatal(err)
	}

	if _, err := session.Request(action.Shoot{}); err != nil {
		t.Fatal(err)
	}

	request := session.Pending()[0]
	state = session.Snapshot()
	state.Fight.TurnRevision++
	if err := session.Commit(state); err != nil {
		t.Fatal(err)
	}

	if err := session.Validate(request); err == nil {
		t.Fatal("old request survived a repeated turn number")
	}
}
