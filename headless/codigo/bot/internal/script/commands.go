package script

import (
	lua "github.com/yuin/gopher-lua"

	"ddbot/bot/internal/actionwire"
	"ddbot/bot/internal/game/action"
)

func (r *Runtime) command(build func(*arguments) action.Command) lua.LGFunction {
	return func(l *lua.LState) int {
		args := readArguments(l)
		command := build(args)
		if err := args.finish(); err != nil {
			return fail(l, err)
		}

		return r.request(command)
	}
}

func (r *Runtime) noArguments(command action.Command) lua.LGFunction {
	return func(l *lua.LState) int {
		if l.GetTop() != 0 {
			return fail(l, &action.InvalidCommandError{Name: command.Name(), Reason: "expected no arguments"})
		}

		return r.request(command)
	}
}

func (r *Runtime) request(command action.Command) int {
	if r.loading {
		return fail(r.state, &action.UnavailableError{Reason: "commands cannot run during script initialization"})
	}

	if _, err := actionwire.Encode(0, command); err != nil {
		return fail(r.state, err)
	}

	id, err := r.session.Request(command)
	if err != nil {
		return fail(r.state, err)
	}

	return r.push(id)
}

func (r *Runtime) fightBindings() map[string]lua.LGFunction {
	return map[string]lua.LGFunction{
		"cards":             func(l *lua.LState) int { return r.push(r.cardState()) },
		"take_card":         r.command(func(a *arguments) action.Command { return action.CardTake{Index: a.i32("index")} }),
		"take_boss_card":    r.command(func(a *arguments) action.Command { return action.CardTakeBoss{Index: a.i32("index")} }),
		"take_extra_card":   r.command(func(a *arguments) action.Command { return action.CardTakeExtra{Index: a.i32("index")} }),
		"finish_cards":      r.noArguments(action.CardFinish{}),
		"finish_boss_cards": r.noArguments(action.CardFinishBoss{}),
		"state":             func(l *lua.LState) int { return r.push(r.fightState()) },
		"actors":            func(l *lua.LState) int { return r.push(r.actors()) },
		"turn":              func(l *lua.LState) int { return r.push(turnFields(r.session.Snapshot().Fight.Turn)) },
		"wind":              func(l *lua.LState) int { return r.push(windFields(r.session.Snapshot().Fight.Wind)) },
		"actor": func(l *lua.LState) int {
			args := &arguments{table: l.NewTable(), used: make(map[string]bool)}
			args.table.RawSetString("id", l.Get(1))
			id := args.i32("id")
			if args.err != nil {
				return fail(l, args.err)
			}

			actor, ok := r.session.Snapshot().Fight.Actors[id]
			if !ok {
				return r.push(nil)
			}

			return r.push(actorFields(actor))
		},
		"self": func(l *lua.LState) int {
			s := r.session.Snapshot()
			if s.Player.ID != nil {
				for _, actor := range s.Fight.Actors {
					if actor.PlayerID != nil && *actor.PlayerID == *s.Player.ID {
						return r.push(actorFields(actor))
					}
				}
			}

			return r.push(nil)
		},
		"shoot": r.command(func(a *arguments) action.Command {
			return action.Shoot{
				X:     a.i32("x"),
				Y:     a.i32("y"),
				Force: a.i32("force"),
				Angle: a.i32("angle"),
			}
		}),
		"face": r.command(func(a *arguments) action.Command { return action.Direction{Direction: a.i32("direction")} }),
		"move_start": r.command(func(a *arguments) action.Command {
			return action.MoveStart{
				Moving:     a.flag("moving"),
				MoveType:   a.i8("move_type"),
				X:          a.i32("x"),
				Y:          a.i32("y"),
				Direction:  a.i8("direction"),
				Living:     a.flag("is_living"),
				TurnNumber: int16(a.integer("turn_number", 16)),
			}
		}),
		"move_stop": r.command(func(a *arguments) action.Command {
			return action.MoveStop{X: a.i32("x"), Y: a.i32("y"), MoveFlag: a.flag("move_flag")}
		}),
		"pass": r.command(func(a *arguments) action.Command { return action.SkipTurn{Argument: a.i8("argument")} }),
		"pow":  r.noArguments(action.Pow{}),
		"use_prop": r.command(func(a *arguments) action.Command {
			return action.UseProp{BagType: a.i8("bag_type"), Place: a.i32("place"), TemplateID: a.i32("template_id")}
		}),
		"load_progress": r.command(func(a *arguments) action.Command { return action.LoadProgress{Progress: a.i32("progress")} }),
		"shoot_tag": r.command(func(a *arguments) action.Command {
			c := action.ShootTag{Started: a.flag("started")}
			if c.Started {
				c.ShotType = a.i8("shot_type")
			}

			return c
		}),
		"mission_start": r.command(func(a *arguments) action.Command { return action.MissionStart{Flag: a.flag("flag")} }),
		"mission_ready": r.command(func(a *arguments) action.Command { return action.MissionReady{Ready: a.flag("ready")} }),
	}
}

func (r *Runtime) roomBindings() map[string]lua.LGFunction {
	return map[string]lua.LGFunction{
		"invite":      r.command(func(a *arguments) action.Command { return action.RoomInvite{PlayerID: a.i32("player_id")} }),
		"invitations": func(l *lua.LState) int { return r.push(r.invitations()) },
		"state":       func(l *lua.LState) int { return r.push(r.roomState()) },
		"start":       r.noArguments(action.RoomStart{}),
		"leave":       r.noArguments(action.RoomLeave{}),
		"ready":       r.command(func(a *arguments) action.Command { return action.RoomReady{State: a.i8("state")} }),
		"create": r.command(func(a *arguments) action.Command {
			return action.RoomCreate{
				RoomType:  a.i8("room_type"),
				TimeType:  a.i8("time_type"),
				NameValue: a.text("name"),
				Password:  a.text("password"),
			}
		}),
		"join": r.command(func(a *arguments) action.Command {
			c := action.RoomJoin{LoginFlag: a.flag("login_flag"), RoomType: a.i32("room_type"), RoomID: a.i32("room_id")}
			if c.RoomID == -1 {
				c.Argument = a.i32("argument")
				c.Password = a.text("password")
			}

			return c
		}),
		"setup": r.command(func(a *arguments) action.Command {
			return action.RoomSetup{
				MapID:       a.i32("map_id"),
				RoomType:    a.i8("room_type"),
				Password:    a.text("password"),
				NameValue:   a.text("name"),
				TimeType:    a.i8("time_type"),
				Difficulty:  a.i8("difficulty"),
				LevelLimits: a.i32("level_limits"),
				Flag1:       a.flag("flag_1"),
				Flag2:       a.flag("flag_2"),
			}
		}),
	}
}
