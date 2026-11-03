package script

import (
	"math"

	lua "github.com/yuin/gopher-lua"

	"ddbot/bot/internal/game/action"
)

func (a *arguments) integerList(name string, limit int) []int32 {
	table, ok := a.get(name).(*lua.LTable)
	if !ok || table.Len() == 0 || table.Len() > limit {
		a.err = &action.InvalidCommandError{Reason: name + " must be a bounded nonempty array"}
		return nil
	}

	result := make([]int32, table.Len())
	count := 0
	table.ForEach(func(key, value lua.LValue) {
		index, validKey := key.(lua.LNumber)
		number, validValue := value.(lua.LNumber)
		if !validKey || math.IsNaN(float64(index)) || index < 1 || index > lua.LNumber(len(result)) || math.Trunc(float64(index)) != float64(index) {
			a.err = &action.InvalidCommandError{Reason: name + " must have consecutive integer keys"}
			return
		}

		if !validValue || math.IsNaN(float64(number)) || number < math.MinInt32 || number > math.MaxInt32 || math.Trunc(float64(number)) != float64(number) {
			a.err = &action.InvalidCommandError{Reason: name + " must contain int32 values"}
			return
		}

		result[int(index)-1] = int32(number)
		count++
	})
	if count != len(result) {
		a.err = &action.InvalidCommandError{Reason: name + " must not contain holes"}
	}

	return result
}
