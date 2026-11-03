package script

import (
	"context"
	"io"
	"testing"
	"time"

	"ddbot/bot/internal/game"
)

func TestLibraryRequiresOnlyProvidedModules(t *testing.T) {
	options := Options{Library: []Module{
		{Name: "data.value", Source: []byte(`return {value=21}`)},
		{Name: "counter", Source: []byte(`local data=require("data.value"); return {value=data.value*2}`)},
	}}

	for i := 0; i < 2; i++ {
		runtime, err := NewRuntimeWithOptions(game.NewSession(1, false), []byte(`return {api_version=1}`), io.Discard, time.Second, options)
		if err != nil {
			t.Fatal(err)
		}

		got, err := runtime.Evaluate(context.Background(), `local c=require("counter"); assert(c==require("counter")); local n=c.value;c.value=0;return n`)
		if err != nil || got != float64(42) {
			t.Fatalf("module = %#v, %v", got, err)
		}

		for _, source := range []string{`return require("os")`, `return require("../outside")`, `return require("data/missing")`} {
			if _, err := runtime.Evaluate(context.Background(), source); err == nil {
				t.Fatalf("accepted %s", source)
			}
		}

		runtime.Close()
	}
}

func TestLibraryRejectsCyclesAndRetriesFailures(t *testing.T) {
	options := Options{Library: []Module{{Name: "a", Source: []byte(`return require("b")`)}, {Name: "b", Source: []byte(`return require("a")`)}}}
	runtime, err := NewRuntimeWithOptions(game.NewSession(1, false), []byte(`return {api_version=1}`), io.Discard, time.Second, options)
	if err != nil {
		t.Fatal(err)
	}

	defer runtime.Close()

	for i := 0; i < 2; i++ {
		got, err := runtime.Evaluate(context.Background(), `local ok,e=pcall(require,"a");assert(not ok);return e.code`)
		if err != nil || got != "module_cycle" {
			t.Fatalf("cycle = %#v, %v", got, err)
		}
	}
}
