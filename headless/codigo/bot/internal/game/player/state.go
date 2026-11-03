package player

import (
	"maps"

	"ddbot/bot/internal/game/value"
)

type State struct {
	BagLock            *BagLock
	ID                 *int32
	InventoryUncertain bool
	InventoryRevision  uint64
	BagRevisions       map[int32]uint64
	BuffsKnown         bool
	Inventory          map[int32]map[int32]Item
	Buffs              map[int32]Buff
}

func (s State) Clone() State {
	next := State{ID: value.Copy(s.ID), BagLock: value.Copy(s.BagLock), Buffs: maps.Clone(s.Buffs), InventoryUncertain: s.InventoryUncertain, InventoryRevision: s.InventoryRevision, BagRevisions: maps.Clone(s.BagRevisions), BuffsKnown: s.BuffsKnown}
	if s.Inventory != nil {
		next.Inventory = make(map[int32]map[int32]Item, len(s.Inventory))
		for bag, slots := range s.Inventory {
			next.Inventory[bag] = maps.Clone(slots)
		}
	}

	return next
}
