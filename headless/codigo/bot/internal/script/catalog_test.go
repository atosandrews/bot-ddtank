package script

import (
	"context"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"ddbot/bot/internal/game"
)

func scriptLibrary(t *testing.T) []Module {
	t.Helper()
	var modules []Module

	for _, name := range []string{"catalog", "terrain", "trajectory"} {
		source, err := os.ReadFile("../../lua/" + name + ".lua")
		if err != nil {
			t.Fatal(err)
		}

		modules = append(modules, Module{Name: name, Source: source})
	}

	return append(modules,
		Module{Name: "data.projectiles", Source: []byte(`return {["1"]={id=1,mass=10,weight=70,wind=240,drag=1},["2"]={id=2,mass=10,weight=70,wind=0,drag=1}}`)},
		Module{Name: "data.weapons", Source: []byte(`return {["4"]={id=4,min_angle=15,max_angle=65,common=1,special=2,added_damage=2,multi=2}}`)},
		Module{Name: "data.items", Source: []byte(`local item=require("data.items.resources.test_weapon");return {[tostring(item.id)]=item}`)},
		Module{Name: "data.items.resources.test_weapon", Source: []byte(`return {id=4,name="Test weapon",category_id=7,properties={[1]="1"}}`)},
		Module{Name: "data.maps", Source: []byte(`return {["3"]={id=3,gravity=10,drag=2,wind_rate=1,width=1000,height=600,foreground="",dead="dead",terrain="data.terrain.3"},["5"]={id=5,foreground="fore"},["6"]={id=6,foreground=""}}`)},
		Module{Name: "data.terrain.3", Source: []byte(`local rows={};for y=500,599 do rows[y]={0,1000} end;return {width=1000,height=600,rows=rows}`)},
	)
}

func TestLuaCatalogAndTerrain(t *testing.T) {
	options := Options{Library: scriptLibrary(t)}
	runtime, err := NewRuntimeWithOptions(game.NewSession(1, false), []byte(`return {api_version=1}`), io.Discard, time.Second, options)
	if err != nil {
		t.Fatal(err)
	}

	defer runtime.Close()
	got, err := runtime.Evaluate(context.Background(), `local c=require("catalog");local t=require("terrain");local p=c.projectile({id=1});p.mass=1;return {mass=c.projectile({id=1}).mass,common=c.weapon({id=4}).common,width=c.map({id=3}).width,solid=t.solid({map_id=3,x=50,y=500}),name=c.item({id=4}).name}`)
	want := map[string]any{
		"mass":   float64(10),
		"common": float64(1),
		"width":  float64(1000),
		"solid":  true,
		"name":   "Test weapon",
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("value = %#v, %v", got, err)
	}

	for _, source := range []string{
		`return require("catalog").projectile({id=99})`,
		`return require("terrain").solid({map_id=3,x=0/0,y=5})`,
		`return require("catalog").map({id=3,typo=1})`,
		`return require("terrain").trace({map_id=3,x=0,y=0,to_x=1000000,to_y=0})`,
		`return require("terrain").solid({map_id=5,x=0,y=0})`,
		`return require("terrain").solid({map_id=6,x=0,y=0})`,
	} {
		if _, err := runtime.Evaluate(context.Background(), source); err == nil {
			t.Fatalf("accepted %s", source)
		}
	}

	_, err = runtime.Evaluate(context.Background(), `return require("catalog").map({id=99})`)
	if err == nil || err.Error() != "lua evaluate: catalog_unavailable: maps record absent: 99" {
		t.Fatalf("error = %v", err)
	}
}

func TestLuaModuleLoadFailures(t *testing.T) {
	for _, modules := range [][]Module{
		{{Name: "fight", Source: []byte(`return {}`)}},
		{{Name: "x", Source: []byte(`return 1`)}},
		{{Name: "x", Source: []byte(`while true do end`)}},
		{{Name: "x", Source: []byte(`return {}`)}, {Name: "x", Source: []byte(`return {}`)}},
	} {
		runtime, err := NewRuntimeWithOptions(game.NewSession(1, false), []byte(`return {api_version=1}`), io.Discard, 20*time.Millisecond, Options{Modules: modules})
		if runtime != nil {
			runtime.Close()
		}

		if err == nil {
			t.Fatal("invalid module accepted")
		}
	}
}

func TestLuaTerrainSpansAndSegmentContacts(t *testing.T) {
	library := scriptLibrary(t)

	for i := range library {
		if library[i].Name == "data.terrain.3" {
			library[i].Source = []byte(`return {width=1000,height=600,rows={[5]={2,4,7,9},[6]={2,4}}}`)
		}
	}

	runtime, err := NewRuntimeWithOptions(game.NewSession(1, false), []byte(`return {api_version=1}`), io.Discard, time.Second, Options{Library: library})
	if err != nil {
		t.Fatal(err)
	}

	defer runtime.Close()

	for _, tc := range []struct {
		name, source string
		want         any
	}{
		{"hole", `return require("terrain").solid({map_id=3,x=4,y=5})`, false},
		{"second span", `return require("terrain").solid({map_id=3,x=8,y=5})`, true},
		{"span end", `return require("terrain").solid({map_id=3,x=9,y=5})`, false},
		{"outside", `return require("terrain").solid({map_id=3,x=-1,y=5})`, false},
		{"forward", `return require("terrain").trace({map_id=3,x=0,y=5,to_x=10,to_y=5})`, map[string]any{
			"hit":      true,
			"x":        float64(2),
			"y":        float64(5),
			"fraction": float64(0.2),
		}},
		{"backward", `return require("terrain").trace({map_id=3,x=10,y=5,to_x=0,to_y=5})`, map[string]any{
			"hit":      true,
			"x":        float64(9),
			"y":        float64(5),
			"fraction": float64(0.1),
		}},
		{"stationary empty", `return require("terrain").trace({map_id=3,x=4,y=5,to_x=4,to_y=5})`, map[string]any{
			"hit":      false,
			"x":        float64(4),
			"y":        float64(5),
			"fraction": float64(1),
		}},
		{"stationary solid", `return require("terrain").trace({map_id=3,x=2,y=5,to_x=2,to_y=5})`, map[string]any{
			"hit":      true,
			"x":        float64(2),
			"y":        float64(5),
			"fraction": float64(0),
		}},
		{"endpoint", `return require("terrain").trace({map_id=3,x=2,y=4,to_x=2,to_y=5})`, map[string]any{
			"hit":      true,
			"x":        float64(2),
			"y":        float64(5),
			"fraction": float64(1),
		}},
		{"diagonal", `return require("terrain").trace({map_id=3,x=1,y=4,to_x=3,to_y=6})`, map[string]any{
			"hit":      true,
			"x":        float64(2),
			"y":        float64(5),
			"fraction": float64(0.5),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := runtime.Evaluate(context.Background(), tc.source)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("result = %#v, %v; want %#v", got, err, tc.want)
			}
		})
	}
}
