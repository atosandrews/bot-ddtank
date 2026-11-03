package game

import (
	"testing"

	"ddbot/bot/internal/game/action"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/value"
)

func TestCardCommandFences(t *testing.T) {
	s := NewSession(1, true)
	next := s.Snapshot()
	next.Player.ID = value.Of(int32(7))
	next.Fight.Phase = "ended"
	next.Fight.Cards = fight.Cards{
		Revision: 1,
		Known:    true,
		Type:     2,
		Players:  map[int32]fight.CardPlayer{7: {PlayerID: 7, CardCount: 2}},
	}
	if err := s.Commit(next); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Request(action.CardTake{Index: 1}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Request(action.CardTake{Index: 1}); err == nil {
		t.Fatal("duplicate queued card accepted")
	}

	if _, err := s.Request(action.CardTakeBoss{Index: 1}); err == nil {
		t.Fatal("wrong board accepted")
	}

	pending := s.Pending()
	if err := s.Validate(pending[0]); err != nil {
		t.Fatal(err)
	}

	next = s.Snapshot()
	next.Fight.Cards.Revision++
	if err := s.Commit(next); err != nil {
		t.Fatal(err)
	}

	if err := s.Validate(pending[0]); err == nil {
		t.Fatal("stale card round accepted")
	}

	next = s.Snapshot()
	next.Fight.Cards.Requested = map[int32]string{2: "extra"}
	next.Fight.Cards.Awards = map[int32]fight.CardAward{3: {Index: 3, PlayerID: 7}}
	if err := s.Commit(next); err != nil {
		t.Fatal(err)
	}

	for _, index := range []int32{2, 3} {
		if _, err := s.Request(action.CardTakeExtra{Index: index}); err == nil {
			t.Fatalf("unavailable slot %d accepted", index)
		}
	}

	next = s.Snapshot()
	next.Fight.Cards.Closed = true
	if err := s.Commit(next); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Request(action.CardFinish{}); err == nil {
		t.Fatal("closed board accepted")
	}
}
