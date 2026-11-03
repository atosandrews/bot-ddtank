package script

import (
	"errors"
	"fmt"

	lua "github.com/yuin/gopher-lua"

	"ddbot/bot/internal/game/action"
)

type Error struct {
	Operation, Reason string
	Cause             error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		var apiError *lua.ApiError
		if errors.As(e.Cause, &apiError) {
			if details, ok := apiError.Object.(*lua.LTable); ok {
				code, validCode := details.RawGetString("code").(lua.LString)
				message, validMessage := details.RawGetString("message").(lua.LString)
				if validCode && validMessage {
					return fmt.Sprintf("lua %s: %s: %s", e.Operation, code, message)
				}
			}
		}

		return fmt.Sprintf("lua %s: %v", e.Operation, e.Cause)
	}

	return "lua " + e.Operation + ": " + e.Reason
}

func (e *Error) Unwrap() error { return e.Cause }

func fail(state *lua.LState, err error) int {
	code := "invalid_argument"
	var unavailable *action.UnavailableError
	if errors.As(err, &unavailable) {
		code = "command_unavailable"
	}

	table := state.NewTable()
	table.RawSetString("code", lua.LString(code))
	table.RawSetString("message", lua.LString(err.Error()))
	state.Error(table, 1)

	return 0
}
