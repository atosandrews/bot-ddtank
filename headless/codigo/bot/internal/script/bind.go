package script

import (
	"fmt"
	"math"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"ddbot/bot/internal/game/action"
)

func (r *Runtime) bind() {
	r.module("session", map[string]lua.LGFunction{"now": r.now, "report": r.reportStatus, "info": func(l *lua.LState) int {
		s := r.session.View()

		return r.push(map[string]any{
			"stream":       s.Stream,
			"revision":     s.Revision,
			"valid":        s.Valid,
			"can_send":     r.session.CanSend(),
			"last_message": messageFields(s.Message),
		})
	}})
	r.module("player", map[string]lua.LGFunction{
		"organize_inventory": r.command(func(a *arguments) action.Command {
			return action.OrganizeInventory{BagType: a.i32("bag_type"), Places: a.integerList("places", 256), Revision: uint64(a.integer("revision", 54))}
		}),
		"bag_lock":        func(l *lua.LState) int { return r.push(r.bagLock()) },
		"collect_rewards": r.noArguments(action.CollectTemporaryRewards{}),
		"unlock_bag":      r.command(func(a *arguments) action.Command { return action.UnlockBag{Password: a.text("password")} }),
		"inventory":       func(l *lua.LState) int { return r.push(r.inventory()) },
		"buffs":           func(l *lua.LState) int { return r.push(r.playerBuffs()) },
		"state": func(l *lua.LState) int {
			player := r.session.View().Player
			return r.push(map[string]any{
				"id": optional(player.ID), "inventory_uncertain": player.InventoryUncertain,
				"inventory_revision": player.InventoryRevision, "buffs_known": player.BuffsKnown,
				"bag_revisions": r.bagRevisions(),
			})
		}})
	r.module("friends", r.friendBindings())
	r.module("fight", r.fightBindings())
	r.module("room", r.roomBindings())
	r.module("scene", r.sceneBindings())
	r.module("captcha", r.captchaBindings())
	r.module("actions", map[string]lua.LGFunction{"get": func(l *lua.LState) int {
		id, ok := l.Get(1).(lua.LNumber)
		if !ok || id <= 0 || id > maxExactInteger || math.Trunc(float64(id)) != float64(id) {
			return fail(l, &action.InvalidCommandError{Reason: "action id must be a positive exact integer"})
		}

		result, err := r.session.Action(uint64(id))
		if err != nil {
			return fail(l, err)
		}

		return r.push(map[string]any{
			"id":     result.ID,
			"name":   result.Name,
			"status": result.Status,
			"reason": result.Reason,
		})
	}})
	r.module("console", map[string]lua.LGFunction{"log": func(l *lua.LState) int {
		parts := make([]string, 0, l.GetTop())

		for i := 1; i <= l.GetTop(); i++ {
			parts = append(parts, l.Get(i).String())
		}

		if _, err := fmt.Fprintln(r.output, strings.Join(parts, " ")); err != nil {
			return fail(l, &Error{Operation: "output", Cause: err})
		}

		return 0
	}})
}

func (r *Runtime) module(name string, functions map[string]lua.LGFunction) {
	table := r.state.NewTable()
	r.state.SetFuncs(table, functions)
	r.state.SetGlobal(name, table)
}
