package script

import (
	"regexp"

	lua "github.com/yuin/gopher-lua"
)

var moduleName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Module struct {
	Name   string
	Source []byte
}

func (r *Runtime) loadModules(modules []Module) error {
	if len(modules) > 16 {
		return &Error{Operation: "modules", Reason: "at most 16 modules are supported"}
	}

	for _, module := range modules {
		if !moduleName.MatchString(module.Name) || r.state.GetGlobal(module.Name) != lua.LNil || len(module.Source) > MaxModuleBytes {
			return &Error{Operation: "module", Reason: "invalid or occupied name, or source exceeds 8 MiB: " + module.Name}
		}

		fn, err := r.state.LoadString(string(module.Source))
		if err != nil {
			return &Error{Operation: "compile module " + module.Name, Cause: err}
		}

		if err := r.state.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}); err != nil {
			return &Error{Operation: "load module " + module.Name, Cause: err}
		}

		table, ok := r.state.Get(-1).(*lua.LTable)
		if !ok {
			return &Error{Operation: "load module " + module.Name, Reason: "module must return a table"}
		}

		r.state.Pop(1)
		r.state.SetGlobal(module.Name, table)
	}

	return nil
}
