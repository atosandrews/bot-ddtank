package script

import lua "github.com/yuin/gopher-lua"

type hooks struct {
	event  *lua.LFunction
	packet *lua.LFunction
	close  *lua.LFunction
	tick   *lua.LFunction
	action *lua.LFunction
}

func readHooks(state *lua.LState) (*hooks, error) {
	if state.GetTop() != 1 {
		return nil, &Error{Operation: "load", Reason: "return one hook table"}
	}

	table, ok := state.Get(-1).(*lua.LTable)
	if !ok {
		return nil, &Error{Operation: "load", Reason: "return a hook table"}
	}

	if table.RawGetString("api_version") != lua.LNumber(1) {
		return nil, &Error{Operation: "load", Reason: "api_version must be 1"}
	}

	registered := &hooks{}
	var failure error
	table.ForEach(func(key, value lua.LValue) {
		if failure != nil {
			return
		}

		if key == lua.LString("api_version") {
			return
		}

		function, ok := value.(*lua.LFunction)
		if !ok {
			failure = &Error{Operation: "load", Reason: "hook must be a function: " + key.String()}

			return
		}

		switch key {
		case lua.LString("on_event"):
			registered.event = function
		case lua.LString("on_packet"):
			registered.packet = function
		case lua.LString("on_close"):
			registered.close = function
		case lua.LString("on_tick"):
			registered.tick = function
		case lua.LString("on_action"):
			registered.action = function
		default:
			failure = &Error{Operation: "load", Reason: "unsupported hook: " + key.String()}
		}
	})
	if failure != nil {
		return nil, failure
	}

	return registered, nil
}
