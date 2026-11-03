package game

import (
	"ddbot/api/contract"
	"ddbot/bot/internal/game/action"
)

func (s *Session) validateInventoryOrganization(command action.OrganizeInventory) error {
	if err := contract.ValidateInventoryOrganization(command.BagType, command.Places); err != nil {
		return &action.InvalidCommandError{Name: command.Name(), Reason: err.Error()}
	}

	if s.state.Player.InventoryUncertain || command.Revision != s.state.Player.InventoryRevision {
		return &action.UnavailableError{Reason: "inventory changed or is incomplete"}
	}

	if s.state.Fight.Phase == "active" || s.state.Fight.Phase == "loading" {
		return &action.UnavailableError{Reason: "inventory organization requires an idle reward or room boundary"}
	}

	start := int32(0)
	if command.BagType == 0 {
		start = 31
	}

	bag := s.state.Player.Inventory[command.BagType]
	count := 0
	for place := range bag {
		if place >= start {
			count++
		}
	}

	if count != len(command.Places) {
		return &action.UnavailableError{Reason: "organization must include every observed unequipped slot"}
	}

	for _, place := range command.Places {
		if _, exists := bag[place]; !exists {
			return &action.UnavailableError{Reason: "organization references an empty slot"}
		}
	}

	return nil
}
