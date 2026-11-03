package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"

	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/action"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/value"
	"ddbot/bot/internal/script"
)

func TestExternalSoloScript(t *testing.T) {
	testPath := os.Getenv("DDTANK_SOLO_TESTS")
	if testPath == "" {
		t.Skip("external Lua suite not configured")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(testPath), "../.."))
	if err != nil {
		t.Fatal(err)
	}
	l := lua.NewState()
	defer l.Close()

	args := l.NewTable()
	args.Append(lua.LString(root))
	l.SetGlobal("arg", args)
	if err := l.DoFile(testPath); err != nil {
		t.Fatal(err)
	}

	source, err := os.ReadFile(root + "/scripts/formigueiro_easy.lua")
	if err != nil {
		t.Fatal(err)
	}

	options, err := loadLibrary(Config{LuaDirectory: "../../lua"})
	if err != nil {
		t.Fatal(err)
	}

	for _, wind := range []int32{-60, 0, 60} {
		session := game.NewSession(1, true)
		next := session.Snapshot()
		next.Player.ID = value.Of(int32(7))
		next.Fight.Generation = 2
		next.Fight.Phase = "active"
		next.Fight.MapID = value.Of(int32(1120))
		next.Fight.TurnRevision = 1
		next.Fight.Turn = &fight.Turn{ActorID: 0, Number: 1, Time: 30}
		next.Fight.Wind = fight.Wind{Current: true, Tenths: value.Of(wind)}
		next.Fight.Actors[0] = fight.Actor{
			ID: 0, PlayerID: value.Of(int32(7)), Team: value.Of(int32(1)),
			WeaponID: value.Of(int32(7001)), Direction: value.Of(int32(-1)),
			HP: value.Of(int32(1303)), Alive: value.Of(true), Position: &fight.Position{X: 680, Y: 504},
		}
		next.Fight.Actors[1] = fight.Actor{
			ID: 1, Team: value.Of(int32(2)), HP: value.Of(int32(500)), MaxHP: value.Of(int32(500)),
			Position: &fight.Position{X: 253, Y: 507},
		}
		if err := session.Commit(next); err != nil {
			t.Fatal(err)
		}

		runtime, err := script.NewRuntimeWithOptions(session, source, io.Discard, time.Second, options)
		if err != nil {
			t.Fatal(err)
		}

		started := time.Now()
		err = runtime.Packet(context.Background(), map[string]any{
			"direction": "ServerToClient", "origin": "wire", "status": "decoded",
			"code": 91, "fields": map[string]any{"subcommand": 6}, "time": "2026-10-23T18:00:00Z",
		})
		if err != nil {
			runtime.Close()
			t.Fatal(err)
		}

		pending := session.Pending()
		if len(pending) != 3 {
			status, err := runtime.Evaluate(context.Background(), "return formigueiro_status()")
			runtime.Close()
			t.Fatalf("wind %d pending=%d status=%v error=%v", wind, len(pending), status, err)
		}

		if _, ok := pending[2].Command.(action.Shoot); !ok {
			runtime.Close()
			t.Fatalf("last command is %T", pending[2].Command)
		}

		t.Logf("wind %d: real runtime shot calculation %s", wind, time.Since(started))
		runtime.Close()
	}
}
