package script

import (
	"context"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func (r *Runtime) Tick(ctx context.Context) error {
	return r.call(ctx, "on_tick", r.hooks.tick, map[string]any{"elapsed": time.Since(r.started).Seconds()})
}

func (r *Runtime) now(l *lua.LState) int {
	l.Push(lua.LNumber(time.Since(r.started).Seconds()))

	return 1
}
