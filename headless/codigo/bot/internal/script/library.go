package script

import (
	"regexp"

	lua "github.com/yuin/gopher-lua"
)

const MaxModuleBytes = 8 << 20
const MaxLibraryBytes = 32 << 20
const MaxLibraryModules = 16384

var libraryName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)*$`)

func (r *Runtime) bindLibrary(modules []Module) error {
	if len(modules) > MaxLibraryModules {
		return &Error{Operation: "library", Reason: "too many modules"}
	}

	sources := make(map[string][]byte, len(modules))
	total := 0

	for _, module := range modules {
		total += len(module.Source)
		_, duplicate := sources[module.Name]
		if !libraryName.MatchString(module.Name) || duplicate || len(module.Source) > MaxModuleBytes || total > MaxLibraryBytes {
			return &Error{Operation: "library", Reason: "invalid or duplicate name, or source limit exceeded: " + module.Name}
		}

		sources[module.Name] = module.Source
	}

	loaded := make(map[string]*lua.LTable)
	loading := make(map[string]bool)
	r.state.SetGlobal("require", r.state.NewFunction(func(state *lua.LState) int {
		name := state.CheckString(1)
		if table, ok := loaded[name]; ok {
			state.Push(table)

			return 1
		}

		source, ok := sources[name]
		if !ok {
			return moduleFailure(state, "module_unavailable", "module not provided: "+name)
		}

		if loading[name] {
			return moduleFailure(state, "module_cycle", "circular dependency: "+name)
		}

		fn, err := state.LoadString(string(source))
		if err != nil {
			return moduleFailure(state, "module_invalid", err.Error())
		}

		loading[name] = true
		err = state.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true})
		delete(loading, name)
		if err != nil {
			if api, ok := err.(*lua.ApiError); ok {
				state.Error(api.Object, 1)

				return 0
			}

			return moduleFailure(state, "module_invalid", err.Error())
		}

		table, ok := state.Get(-1).(*lua.LTable)
		if !ok {
			return moduleFailure(state, "module_invalid", "module must return a table: "+name)
		}

		loaded[name] = table

		return 1
	}))

	return nil
}

func moduleFailure(state *lua.LState, code, message string) int {
	table := state.NewTable()
	table.RawSetString("code", lua.LString(code))
	table.RawSetString("message", lua.LString(message))
	state.Error(table, 1)

	return 0
}
