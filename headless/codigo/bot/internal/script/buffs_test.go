package script

import (
	"context"
	"io"
	"testing"
	"time"

	"ddbot/bot/internal/actionwire"
	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/player"
	"ddbot/bot/internal/game/value"
)

func TestPowLuaContractAndQueries(t *testing.T) {
	session := game.NewSession(1, true)
	s := session.Snapshot()
	s.Player.ID = value.Of(int32(7))
	s.Player.Inventory = map[int32]map[int32]player.Item{3: {2: {BagType: 3, Place: 2, TemplateID: 10004, Count: 1}}}
	s.Player.Buffs = map[int32]player.Buff{15: {Type: 15}}
	s.Fight.Phase = "active"
	s.Fight.Turn = &fight.Turn{ActorID: 0}
	s.Fight.Actors[0] = fight.Actor{ID: 0, PlayerID: value.Of(int32(7)), Energy: value.Of(float64(240)), Dander: value.Of(int32(200))}
	if err := session.Commit(s); err != nil {
		t.Fatal(err)
	}

	runtime, err := NewRuntime(session, []byte(`return {api_version=1}`), io.Discard, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	result, err := runtime.Evaluate(context.Background(), `
        assert(player.inventory()[1].bag_type==3 and player.inventory()[1].place==2)
        assert(player.buffs()[1].type==15)
        assert(fight.self().energy==240)
        player.inventory()[1].count=0
        assert(player.inventory()[1].count==1)
        return fight.pow()`)
	if err != nil || result != float64(1) {
		t.Fatalf("result=%v err=%v", result, err)
	}

	pending := session.Pending()
	if len(pending) != 1 || pending[0].Command.Name() != "pow" {
		t.Fatal("POW not queued")
	}

	command, err := actionwire.Encode(1, pending[0].Command)
	if err != nil || command.GetPow() == nil {
		t.Fatalf("POW contract: %v", err)
	}

	s = session.Snapshot()
	s.Fight.Turn.ActorID = 1
	if err := session.Commit(s); err != nil {
		t.Fatal(err)
	}

	result, err = runtime.Evaluate(context.Background(), `local ok=pcall(fight.pow); return ok`)
	if err != nil || result != false {
		t.Fatalf("foreign turn POW=%v err=%v", result, err)
	}
}
