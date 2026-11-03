package projection

import (
	"math"
	"strconv"
	"strings"

	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/value"
)

func numericProperty(name, raw string) (float64, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return 0, nil
	}

	if strings.Contains(text, "_") {
		return 0, &NumericPropertyError{Property: name, Reason: "unsupported numeric value"}
	}

	var number float64
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		integer, err := strconv.ParseUint(text[2:], 16, 53)
		if err != nil {
			return 0, &NumericPropertyError{Property: name, Reason: "unsupported numeric value", Cause: err}
		}

		number = float64(integer)
	} else {
		parsed, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, &NumericPropertyError{Property: name, Reason: "unsupported numeric value", Cause: err}
		}

		number = parsed
	}

	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, &NumericPropertyError{Property: name, Reason: "non-finite value"}
	}

	return number, nil
}

func projectFightProperty(actor *fight.Actor, name, raw string) {
	enabled := raw != "false" && raw != "False" && raw != ""
	controls := &actor.Controls
	switch name {
	case "system", "silencedSpecial":
		controls.LockType = 0
		if name == "silencedSpecial" {
			controls.LockType = 3
		}

		controls.LockState = enabled
		controls.LockSpellKill = enabled
		controls.RightPropEnabled = !enabled
	case "silenced":
		controls.LockType = 1
		controls.LockState = enabled
		controls.RightPropEnabled = !enabled
	case "propzxc":
		controls.LockType = 3
		controls.LockState = enabled
	case "nofly":
		controls.LockType = 2
		controls.LockState = enabled
	case "silenceMany":
		controls.LockRightProp = enabled
	case "energy", "energy2":
		number, err := numericProperty(name, raw)
		if err != nil {
			actor.Energy = nil
			actor.EnergyIssue = value.Of(err.Error())
			if name == "energy" {
				actor.MaxEnergy = nil
			}

			return
		}

		if name == "energy" {
			actor.MaxEnergy = value.Of(number)
		}

		if actor.MaxEnergy != nil && number > *actor.MaxEnergy {
			return
		}

		actor.Energy = value.Of(math.Max(0, number))
		actor.EnergyIssue = nil
	default:
		return
	}
}
