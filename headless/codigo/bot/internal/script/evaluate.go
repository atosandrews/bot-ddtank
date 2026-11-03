package script

import (
	"context"

	lua "github.com/yuin/gopher-lua"
)

func (r *Runtime) Evaluate(ctx context.Context, source string) (any, error) {
	if len(source) > 65536 {
		return nil, &Error{Operation: "evaluate", Reason: "source exceeds 64 KiB"}
	}

	deadline, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	r.state.SetContext(deadline)
	defer r.state.RemoveContext()
	defer r.state.SetTop(0)

	function, err := r.state.LoadString(source)
	if err != nil {
		return nil, &Error{Operation: "compile", Cause: err}
	}

	if err := r.state.CallByParam(lua.P{Fn: function, NRet: 1, Protect: true}); err != nil {
		return nil, &Error{Operation: "evaluate", Cause: err}
	}

	budget := 262144

	return resultValue(r.state.Get(-1), make(map[*lua.LTable]bool), 0, &budget)
}

func resultValue(value lua.LValue, visited map[*lua.LTable]bool, depth int, budget *int) (any, error) {
	*budget -= 16
	if depth > 32 || *budget < 0 {
		return nil, &Error{Operation: "result", Reason: "result exceeds size or depth limit"}
	}

	switch v := value.(type) {
	case *lua.LNilType:
		return nil, nil
	case lua.LBool:
		return bool(v), nil
	case lua.LNumber:
		return float64(v), nil
	case lua.LString:
		*budget -= len(v)
		if *budget < 0 {
			return nil, &Error{Operation: "result", Reason: "result exceeds size limit"}
		}
		return string(v), nil
	case *lua.LTable:
		if visited[v] {
			return nil, &Error{Operation: "result", Reason: "cyclic table"}
		}
		visited[v] = true
		defer delete(visited, v)
		object := make(map[string]any)
		array := make(map[int]any)
		var failure error
		v.ForEach(func(key, item lua.LValue) {
			if failure != nil {
				return
			}

			converted, err := resultValue(item, visited, depth+1, budget)
			if err != nil {
				failure = err

				return
			}

			switch k := key.(type) {
			case lua.LString:
				*budget -= len(k)
				object[string(k)] = converted
			case lua.LNumber:
				index := int(k)
				if k < 1 || k > 16384 || lua.LNumber(index) != k {
					failure = &Error{Operation: "result", Reason: "table keys must be strings or consecutive array indices"}

					return
				}
				array[index] = converted
			default:
				failure = &Error{Operation: "result", Reason: "unsupported table key type " + key.Type().String()}
			}
		})
		if failure != nil {
			return nil, failure
		}
		if *budget < 0 {
			return nil, &Error{Operation: "result", Reason: "result exceeds size limit"}
		}
		if len(array) == 0 {
			return object, nil
		}
		if len(object) != 0 {
			return nil, &Error{Operation: "result", Reason: "mixed object and array table"}
		}
		values := make([]any, len(array))
		for i := range values {
			item, ok := array[i+1]
			if !ok {
				return nil, &Error{Operation: "result", Reason: "sparse array"}
			}

			values[i] = item
		}
		return values, nil
	default:
		return nil, &Error{Operation: "result", Reason: "unsupported result type " + value.Type().String()}
	}
}
