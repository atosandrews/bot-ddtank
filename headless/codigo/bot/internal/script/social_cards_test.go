package script

import (
	"context"
	"io"
	"reflect"
	"testing"
	"time"

	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/action"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/value"
)

func TestSocialCardLuaCommands(t *testing.T) {
	tests := []struct {
		source  string
		command action.Command
		mode    int32
	}{
		{`friends.add({name="Pal",relation=0,flag_1=false,flag_2=true})`, action.FriendAdd{NameValue: "Pal", Flag2: true}, 2},
		{`room.invite({player_id=42})`, action.RoomInvite{PlayerID: 42}, 2},
		{`fight.take_card({index=20})`, action.CardTake{Index: 20}, 2},
		{`fight.take_boss_card({index=8})`, action.CardTakeBoss{Index: 8}, 1},
		{`fight.take_extra_card({index=3})`, action.CardTakeExtra{Index: 3}, 2},
		{`fight.finish_cards()`, action.CardFinish{}, 2},
		{`fight.finish_boss_cards()`, action.CardFinishBoss{}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			s := game.NewSession(1, true)
			snapshot := s.Snapshot()
			snapshot.Player.ID = value.Of(int32(7))
			snapshot.Room.ID = value.Of(int32(9))
			snapshot.Fight.Phase = "mission_ended"
			snapshot.Fight.Cards = fight.Cards{Known: true, Type: tt.mode, Players: map[int32]fight.CardPlayer{7: {PlayerID: 7, CardCount: 2, Winner: true}}}
			if err := s.Commit(snapshot); err != nil {
				t.Fatal(err)
			}

			r, err := NewRuntime(s, []byte(`return {api_version=1}`), io.Discard, time.Second)
			if err != nil {
				t.Fatal(err)
			}

			defer r.Close()
			got, err := r.Evaluate(context.Background(), "return "+tt.source)
			if err != nil || got != float64(1) {
				t.Fatalf("result=%v err=%v", got, err)
			}

			pending := s.Pending()
			if len(pending) != 1 || !reflect.DeepEqual(pending[0].Command, tt.command) {
				t.Fatalf("pending=%+v", pending)
			}
		})
	}
}

func TestCardQueriesAndPreconditions(t *testing.T) {
	s := game.NewSession(1, true)
	r, err := NewRuntime(s, []byte(`return {api_version=1}`), io.Discard, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	defer r.Close()
	source := `assert(fight.cards().known==false)
 assert(#room.invitations()==0 and #friends.state().players==0)
 local ok,err=pcall(function() fight.take_card({index=0}) end)
 assert(not ok and err.code=="command_unavailable")
 local bad,reason=pcall(function() fight.take_card({index=100}) end)
 assert(not bad and reason.code=="invalid_argument")
 return true`
	got, err := r.Evaluate(context.Background(), source)
	if err != nil || got != true {
		t.Fatalf("result=%v err=%v", got, err)
	}

	if len(s.Pending()) != 0 {
		t.Fatal("unavailable requests queued")
	}
}
