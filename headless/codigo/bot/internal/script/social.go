package script

import (
	"slices"

	lua "github.com/yuin/gopher-lua"

	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/action"
)

func (r *Runtime) friendBindings() map[string]lua.LGFunction {
	return map[string]lua.LGFunction{
		"state": func(l *lua.LState) int { return r.push(r.friendState()) },
		"add": r.command(func(a *arguments) action.Command {
			return action.FriendAdd{
				NameValue: a.text("name"),
				Relation:  a.i32("relation"),
				Flag1:     a.flag("flag_1"),
				Flag2:     a.flag("flag_2"),
			}
		}),
	}
}

func (r *Runtime) friendState() map[string]any {
	state := r.session.View().Social
	ids := make([]int32, 0, len(state.Friends))

	for id := range state.Friends {
		ids = append(ids, id)
	}

	slices.Sort(ids)
	players := make([]map[string]any, 0, len(ids))

	for _, id := range ids {
		f := state.Friends[id]
		players = append(players, map[string]any{
			"id":       f.ID,
			"name":     f.Name,
			"relation": f.Relation,
			"grade":    f.Grade,
			"state":    f.State,
		})
	}

	result := map[string]any{"players": players, "complete": false}
	if state.LastResult != nil {
		result["last_result"] = map[string]any{"success": state.LastResult.Success, "player_id": optional(state.LastResult.PlayerID)}
	}

	if state.Notification != nil {
		result["notification"] = map[string]any{"argument": state.Notification.Argument, "name": state.Notification.Name, "same_city": state.Notification.SameCity}
	}

	return result
}

func (r *Runtime) invitations() []map[string]any {
	rows := make([]map[string]any, 0)

	for _, i := range r.session.View().Social.Invitations {
		rows = append(rows, map[string]any{
			"player_id":      i.PlayerID,
			"room_id":        i.RoomID,
			"map_id":         i.MapID,
			"second_type":    i.SecondType,
			"game_mode":      i.GameMode,
			"difficulty":     i.Difficulty,
			"level_limits":   i.LevelLimits,
			"name":           i.Name,
			"room_name":      i.RoomName,
			"vip":            i.VIP,
			"vip_level":      i.VIPLevel,
			"barrier_number": i.BarrierNumber,
		})
	}

	return rows
}

func messageFields(message *game.Message) any {
	if message == nil {
		return nil
	}

	return map[string]any{"kind": message.Kind, "message": message.Text}
}
