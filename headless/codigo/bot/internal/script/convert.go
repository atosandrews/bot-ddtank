package script

import (
	"fmt"
	"strconv"

	lua "github.com/yuin/gopher-lua"
)

const maxExactInteger = 1<<53 - 1

func (r *Runtime) convert(value any) (lua.LValue, error) {
	switch v := value.(type) {
	case nil:
		return lua.LNil, nil
	case bool:
		return lua.LBool(v), nil
	case string:
		return lua.LString(v), nil
	case int:
		return signed(int64(v)), nil
	case int8:
		return lua.LNumber(v), nil
	case uint8:
		return lua.LNumber(v), nil
	case int16:
		return lua.LNumber(v), nil
	case uint16:
		return lua.LNumber(v), nil
	case int32:
		return lua.LNumber(v), nil
	case uint32:
		return lua.LNumber(v), nil
	case int64:
		return signed(v), nil
	case uint64:
		if v > maxExactInteger {
			return lua.LString(strconv.FormatUint(v, 10)), nil
		}
		return lua.LNumber(v), nil
	case float64:
		return lua.LNumber(v), nil
	case map[string]any:
		return object(r, v)
	case map[string]uint64:
		return object(r, v)
	case []map[string]any:
		return array(r, v)
	case []int32:
		return array(r, v)
	case []int8:
		return array(r, v)
	case []string:
		return array(r, v)
	case []any:
		return array(r, v)
	default:
		return nil, &Error{Operation: "convert", Reason: fmt.Sprintf("unsupported field type %T", value)}
	}
}

func signed(value int64) lua.LValue {
	if value > maxExactInteger || value < -maxExactInteger {
		return lua.LString(strconv.FormatInt(value, 10))
	}

	return lua.LNumber(value)
}

func object[V any](r *Runtime, values map[string]V) (lua.LValue, error) {
	table := r.state.NewTable()

	for key, value := range values {
		converted, err := r.convert(value)
		if err != nil {
			return nil, err
		}

		table.RawSetString(key, converted)
	}

	return table, nil
}

func array[T any](r *Runtime, values []T) (lua.LValue, error) {
	table := r.state.NewTable()

	for _, value := range values {
		converted, err := r.convert(value)
		if err != nil {
			return nil, err
		}

		table.Append(converted)
	}

	return table, nil
}

func optional[T any](value *T) any {
	if value == nil {
		return nil
	}

	return *value
}

func (r *Runtime) push(value any) int {
	converted, err := r.convert(value)
	if err != nil {
		return fail(r.state, err)
	}

	r.state.Push(converted)

	return 1
}
