package script

import (
	"math"

	lua "github.com/yuin/gopher-lua"

	"ddbot/bot/internal/game/action"
)

type arguments struct {
	table *lua.LTable
	used  map[string]bool
	err   error
}

func readArguments(state *lua.LState) *arguments {
	table, ok := state.Get(1).(*lua.LTable)
	a := &arguments{table: table, used: make(map[string]bool)}
	if !ok || state.GetTop() != 1 {
		a.err = &action.InvalidCommandError{Reason: "expected one argument table"}
	}

	return a
}

func (a *arguments) get(name string) lua.LValue {
	a.used[name] = true
	if a.table == nil {
		return lua.LNil
	}

	return a.table.RawGetString(name)
}

func (a *arguments) integer(name string, bits int) int64 {
	v, ok := a.get(name).(lua.LNumber)
	limit := math.Ldexp(1, bits-1)
	if !ok || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || math.Trunc(float64(v)) != float64(v) || float64(v) < -limit || float64(v) >= limit {
		a.err = &action.InvalidCommandError{Reason: name + " must be an integer in its wire range"}

		return 0
	}

	return int64(v)
}

func (a *arguments) i32(name string) int32 { return int32(a.integer(name, 32)) }

func (a *arguments) i8(name string) int8 { return int8(a.integer(name, 8)) }

func (a *arguments) flag(name string) bool {
	v, ok := a.get(name).(lua.LBool)
	if !ok {
		a.err = &action.InvalidCommandError{Reason: name + " must be a boolean"}
	}

	return bool(v)
}

func (a *arguments) text(name string) string {
	v, ok := a.get(name).(lua.LString)
	if !ok {
		a.err = &action.InvalidCommandError{Reason: name + " must be a string"}
	}

	return string(v)
}

func (a *arguments) finish() error {
	if a.table != nil {
		a.table.ForEach(func(key, value lua.LValue) {
			name, ok := key.(lua.LString)
			if !ok || !a.used[string(name)] {
				a.err = &action.InvalidCommandError{Reason: "unknown argument " + key.String()}
			}
		})
	}

	return a.err
}

func (a *arguments) coordinate(name string) float64 {
	v, ok := a.get(name).(lua.LNumber)
	if !ok || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || math.Abs(float64(v)) > 1e6 {
		a.err = &action.InvalidCommandError{Reason: name + " must be finite and within one million pixels"}

		return 0
	}

	return float64(v)
}
