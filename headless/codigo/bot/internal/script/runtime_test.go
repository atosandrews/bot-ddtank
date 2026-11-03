package script

import (
	"context"
	"io"
	"reflect"
	"testing"
	"time"

	"ddbot/bot/internal/game"
)

func TestLuaSeesCommittedDetachedState(t *testing.T) {
	session := game.NewSession(3, false)
	source := []byte(`return {api_version=1,on_event=function(event)
 local s = session.info()
 assert(s.stream == 3 and s.revision == 1 and s.can_send == false)
 local f = fight.state()
 f.phase = "changed"
 assert(fight.state().phase == "unknown")
 local ok, err = pcall(room.start)
 assert(not ok and err.code == "command_unavailable")
 end}`)
	runtime, err := NewRuntime(session, source, io.Discard, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	defer runtime.Close()
	if err := session.Commit(session.Snapshot()); err != nil {
		t.Fatal(err)
	}

	if err := runtime.Event(context.Background(), "state_changed", nil); err != nil {
		t.Fatal(err)
	}
}

func TestLuaQueuesTypedCommands(t *testing.T) {
	session := game.NewSession(4, true)
	runtime, err := NewRuntime(session, []byte(`return {api_version=1,on_event=function(e)
 local id = room.ready({state=1})
 assert(actions.get(id).status == "queued")
 end}`), io.Discard, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	defer runtime.Close()
	if err := runtime.Event(context.Background(), "room_entered", nil); err != nil {
		t.Fatal(err)
	}

	pending := session.Pending()
	if len(pending) != 1 || pending[0].Command.Name() != "room_ready" {
		t.Fatalf("pending = %#v", pending)
	}
}

func TestLuaDeadlineStopsLoop(t *testing.T) {
	runtime, err := NewRuntime(game.NewSession(1, false), []byte(`return {api_version=1,on_event=function() while true do end end}`), io.Discard, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	defer runtime.Close()
	if err := runtime.Event(context.Background(), "test", nil); err == nil {
		t.Fatal("infinite loop was accepted")
	}
}

func TestSceneCommandsRemainUnavailableInReplay(t *testing.T) {
	for _, call := range []string{`scene.set_state({state=1})`, `scene.login({scene_id=2})`} {
		t.Run(call, func(t *testing.T) {
			session := game.NewSession(1, false)
			runtime, err := NewRuntime(session, []byte(`return {api_version=1}`), io.Discard, time.Second)
			if err != nil {
				t.Fatal(err)
			}

			defer runtime.Close()

			value, err := runtime.Evaluate(context.Background(), `local ok,err=pcall(function() `+call+` end); return {ok=ok,code=err.code}`)
			if err != nil {
				t.Fatal(err)
			}

			expected := map[string]any{"ok": false, "code": "command_unavailable"}
			if !reflect.DeepEqual(value, expected) {
				t.Fatalf("result = %#v, want %#v", value, expected)
			}

			if len(session.Pending()) != 0 {
				t.Fatal("read-only evaluation queued a command")
			}
		})
	}
}

func TestLuaRejectsInvalidCommandArguments(t *testing.T) {
	for _, call := range []string{
		`room.ready({state=128})`,
		`room.ready({state=1.5})`,
		`room.ready({state="1"})`,
		`room.ready({state=1,extra=2})`,
		`room.start({})`,
		`scene.login({})`,
		`scene.login({scene_id="2"})`,
		`scene.login({scene_id=2147483648})`,
		`scene.login({scene_id=2,extra=true})`,
		`scene.set_state({state=1.5})`,
		`scene.set_state({state=1},2)`,
		`fight.shoot({x=1,y=2,force=0/0,angle=45})`,
	} {
		t.Run(call, func(t *testing.T) {
			session := game.NewSession(1, true)
			source := `return {api_version=1,on_event=function() local ok,err=pcall(function() ` + call + ` end); assert(not ok and err.code == 'invalid_argument') end}`
			runtime, err := NewRuntime(session, []byte(source), io.Discard, time.Second)
			if err != nil {
				t.Fatal(err)
			}

			defer runtime.Close()
			if err := runtime.Event(context.Background(), "test", nil); err != nil {
				t.Fatal(err)
			}

			if len(session.Pending()) != 0 {
				t.Fatal("invalid call queued a command")
			}
		})
	}
}

func TestLuaPreservesLargeIntegerAndPacketIsolation(t *testing.T) {
	runtime, err := NewRuntime(game.NewSession(1, false), []byte(`return {api_version=1,on_packet=function(p) assert(p.large == '9007199254740993'); p.values[1].hp=0 end}`), io.Discard, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	defer runtime.Close()
	fields := map[string]any{"large": int64(9007199254740993), "values": []map[string]any{{"hp": int32(100)}}}
	if err := runtime.Packet(context.Background(), fields); err != nil {
		t.Fatal(err)
	}

	if fields["values"].([]map[string]any)[0]["hp"] != int32(100) {
		t.Fatal("Lua mutated decoded packet")
	}
}

func TestInteractiveResultsAreBoundedAndStructured(t *testing.T) {
	runtime, err := NewRuntime(game.NewSession(1, false), []byte(`return {api_version=1}`), io.Discard, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	defer runtime.Close()

	for _, source := range []string{`local t={}; t.t=t; return t`, `return function() end`, `return {[1]=1,name='mixed'}`, `return {[3]=1}`, `while true do end`} {
		if _, err := runtime.Evaluate(context.Background(), source); err == nil {
			t.Fatalf("accepted %s", source)
		}
	}

	value, err := runtime.Evaluate(context.Background(), `return {phase='active',actors={1,2}}`)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{"phase": "active", "actors": []any{float64(1), float64(2)}}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("value = %#v", value)
	}
}

func TestLuaReadsBagRevisionsAndInventoryRevision(t *testing.T) {
	session := game.NewSession(7, true)
	state := session.Snapshot()
	state.Player.InventoryRevision = 42
	state.Player.BagRevisions = map[int32]uint64{0: 9, 1: 30, 15: 1 << 60}
	if err := session.Commit(state); err != nil {
		t.Fatal(err)
	}

	runtime, err := NewRuntime(session, []byte(`return {api_version=1}`), io.Discard, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	defer runtime.Close()
	value, err := runtime.Evaluate(context.Background(), `local state = player.state(); return {revision=state.inventory_revision,bag0=state.bag_revisions["0"],bag1=state.bag_revisions["1"],bag15=state.bag_revisions["15"]}`)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{"revision": float64(42), "bag0": float64(9), "bag1": float64(30), "bag15": "1152921504606846976"}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("value = %#v", value)
	}
}
