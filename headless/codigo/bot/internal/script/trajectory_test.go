package script

import (
	"context"
	"fmt"
	"io"
	"math"
	"reflect"
	"testing"
	"time"

	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/value"
)

func trajectoryRuntime(t *testing.T) (*Runtime, *game.Session) {
	t.Helper()
	session := game.NewSession(1, false)
	state := session.Snapshot()
	state.Player.ID = value.Of(int32(5))
	state.Fight.MapID = value.Of(int32(3))
	state.Fight.Phase = "active"
	state.Fight.Turn = &fight.Turn{ActorID: 0, Number: 1}
	state.Fight.Wind = fight.Wind{Current: true, Tenths: value.Of(int32(0))}
	state.Fight.Actors[0] = fight.Actor{
		ID:        0,
		PlayerID:  value.Of(int32(5)),
		WeaponID:  value.Of(int32(4)),
		Position:  &fight.Position{X: 100, Y: 500},
		Direction: value.Of(int32(1)),
		HP:        value.Of(int32(100)),
		Alive:     value.Of(true),
	}
	if err := session.Commit(state); err != nil {
		t.Fatal(err)
	}

	runtime, err := NewRuntimeWithOptions(session, []byte(`return {api_version=1}`), io.Discard, time.Second, Options{Library: scriptLibrary(t), Modules: []Module{{Name: "trajectory", Source: []byte(`return require("trajectory")`)}}})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(runtime.Close)

	return runtime, session
}

func TestTrajectoryClientEquations(t *testing.T) {
	runtime, _ := trajectoryRuntime(t)

	for _, tc := range []struct {
		wind  float64
		ball  int
		vx, x float64
	}{{1, 1, 100.16, 14.0064}, {-1, 1, 98.24, 13.9296}, {5, 2, 99.2, 13.968}} {
		source := `return trajectory.step({x=10,y=10,vx=100,vy=0},trajectory.model({map_id=3,projectile_id=1,wind=1}))`
		if tc.wind == -1 {
			source = `return trajectory.step({x=10,y=10,vx=100,vy=0},trajectory.model({map_id=3,projectile_id=1,wind=-1}))`
		}

		if tc.ball == 2 {
			source = `return trajectory.step({x=10,y=10,vx=100,vy=0},trajectory.model({map_id=3,projectile_id=2,wind=5}))`
		}

		got, err := runtime.Evaluate(context.Background(), source)
		if err != nil {
			t.Fatal(err)
		}

		want := map[string]float64{
			"x":  tc.x,
			"y":  11.12,
			"vx": tc.vx,
			"vy": 28,
		}
		result := got.(map[string]any)
		if len(result) != len(want) {
			t.Fatalf("step = %#v", result)
		}

		for k, v := range want {
			if math.Abs(result[k].(float64)-v) > 1e-10 {
				t.Fatalf("%s = %v, want %v", k, result[k], v)
			}
		}
	}

	got, err := runtime.Evaluate(context.Background(), `return trajectory.launch({x=100,y=500,direction=-1,slope=0,gun_angle=45,force=100})`)
	want := map[string]any{
		"x":     float64(70),
		"y":     float64(477),
		"angle": float64(-135),
		"force": float64(100),
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("launch = %#v, %v", got, err)
	}
}

const searchSource = `job=trajectory.search({actor_id=0,direction=1,target={x=400,y=480,radius=12},force_min=100,force_max=600,force_step=20,angle_min=15,angle_max=65,angle_step=5,max_steps=150}); return true`

func TestTrajectorySearchAndTerrainOcclusion(t *testing.T) {
	runtime, _ := trajectoryRuntime(t)
	if _, err := runtime.Evaluate(context.Background(), searchSource); err != nil {
		t.Fatal(err)
	}

	found := false

	for i := 0; i < 100; i++ {
		result, err := runtime.Evaluate(context.Background(), `return job:step(5)`)
		if err != nil {
			t.Fatal(err)
		}

		if result.(map[string]any)["status"] == "found" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("search did not find reachable target")
	}

	result, err := runtime.Evaluate(context.Background(), `return trajectory.simulate({map_id=3,projectile_id=1,wind=0,shot=job:shot(),target={x=400,y=480,radius=12},max_steps=150}).status`)
	if err != nil || result != "target" {
		t.Fatalf("solution = %#v, %v", result, err)
	}

	result, err = runtime.Evaluate(context.Background(), `return trajectory.simulate({map_id=3,projectile_id=1,wind=0,shot={x=100,y=400,force=300,angle=90},target={x=100,y=550,radius=5},max_steps=100}).status`)
	if err != nil || result != "terrain" {
		t.Fatalf("obstructed target = %#v, %v", result, err)
	}
}

func TestTrajectoryRejectsStaleContext(t *testing.T) {
	for _, kind := range []string{"turn", "wind", "position", "weapon", "ball", "invalid"} {
		t.Run(kind, func(t *testing.T) {
			runtime, session := trajectoryRuntime(t)
			if _, err := runtime.Evaluate(context.Background(), searchSource); err != nil {
				t.Fatal(err)
			}

			state := session.Snapshot()
			actor := state.Fight.Actors[0]

			switch kind {
			case "turn":
				state.Fight.TurnRevision++
			case "wind":
				state.Fight.Wind.Tenths = value.Of(int32(10))
			case "position":
				actor.Position.X++
			case "weapon":
				actor.WeaponID = value.Of(int32(99))
			case "ball":
				actor.BallID = value.Of(int32(2))
			case "invalid":
				state.Valid = false
			default:
				t.Fatalf("unsupported case %s", kind)
			}

			state.Fight.Actors[0] = actor
			if err := session.Commit(state); err != nil {
				t.Fatal(err)
			}

			result, err := runtime.Evaluate(context.Background(), `local ok,e=pcall(function() return job:step(1) end); assert(not ok); return e.code`)
			if err != nil || result != "trajectory_stale" {
				t.Fatalf("stale result = %#v, %v", result, err)
			}
		})
	}
}

func TestTrajectoryLimitsAndMissingInputs(t *testing.T) {
	runtime, session := trajectoryRuntime(t)

	for _, source := range []string{
		`return trajectory.model({map_id=3,projectile_id=87,wind=0})`,
		`return trajectory.launch({x=0,y=0,slope=0,direction=0,gun_angle=45,force=100})`,
		`return trajectory.launch({x=0,y=0,slope=0,direction=1,gun_angle=45,force=2001})`,
		`return trajectory.launch({x=0/0,y=0,slope=0,direction=1,gun_angle=45,force=100})`,
		`return trajectory.simulate({map_id=3,projectile_id=1,wind=0,shot={x=10,y=10,force=100,angle=0},target={x=50,y=50,radius=10},max_steps=751})`,
		`return trajectory.search({actor_id=0,direction=1,target={x=400,y=480,radius=12},force_min=100,force_max=600,force_step=20,angle_min=0,angle_max=65,angle_step=5,max_steps=150})`,
	} {
		if _, err := runtime.Evaluate(context.Background(), source); err == nil {
			t.Fatalf("accepted %s", source)
		}
	}

	if _, err := runtime.Evaluate(context.Background(), `job=trajectory.search({actor_id=0,direction=1,target={x=900,y=100,radius=1},force_min=0,force_max=0,force_step=1,angle_min=15,angle_max=15,angle_step=1,max_steps=20});return true`); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.Evaluate(context.Background(), `local ok,e=pcall(function()return job:step(2)end);assert(not ok);return e.code`)
	if err != nil || result != "trajectory_no_solution" {
		t.Fatalf("no solution = %#v, %v", result, err)
	}

	state := session.Snapshot()
	state.Fight.Wind.Expire()
	if err := session.Commit(state); err != nil {
		t.Fatal(err)
	}

	if _, err := runtime.Evaluate(context.Background(), searchSource); err == nil {
		t.Fatal("unknown wind accepted")
	}
}

func TestTrajectoryJobsAndCatalogMutationStayInTheirVM(t *testing.T) {
	first, firstSession := trajectoryRuntime(t)
	second, _ := trajectoryRuntime(t)
	ctx := context.Background()

	for _, runtime := range []*Runtime{first, second} {
		if _, err := runtime.Evaluate(ctx, searchSource); err != nil {
			t.Fatal(err)
		}
	}

	results := make(chan error, 2)

	for _, entry := range []struct {
		runtime *Runtime
		source  string
		want    float64
	}{{first, `return job:step(3).tested`, 3}, {second, `return job:step(1).tested`, 1}} {
		go func() {
			value, err := entry.runtime.Evaluate(ctx, entry.source)
			if err == nil && value != entry.want {
				err = fmt.Errorf("tested=%v want=%v", value, entry.want)
			}

			results <- err
		}()
	}

	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}

	if _, err := first.Evaluate(ctx, `require("data.maps")["3"].gravity=999;require("data.items")["4"].name="changed";job=nil;return true`); err != nil {
		t.Fatal(err)
	}

	firstSession.Invalidate("test")
	value, err := second.Evaluate(ctx, `local catalog=require("catalog");return {tested=job:step(1).tested,gravity=catalog.map({id=3}).gravity,name=catalog.item({id=4}).name}`)
	want := map[string]any{"tested": float64(2), "gravity": float64(10), "name": "Test weapon"}
	if err != nil || !reflect.DeepEqual(value, want) {
		t.Fatalf("second VM=%v error=%v", value, err)
	}
}
