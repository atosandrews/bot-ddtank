package script

import (
	lua "github.com/yuin/gopher-lua"

	"ddbot/bot/internal/game/action"
)

func (r *Runtime) sceneBindings() map[string]lua.LGFunction {
	return map[string]lua.LGFunction{
		"login": r.command(func(a *arguments) action.Command {
			return action.SceneLogin{SceneID: a.i32("scene_id")}
		}),
		"set_state": r.command(func(a *arguments) action.Command {
			return action.SceneState{State: a.i32("state")}
		}),
	}
}
