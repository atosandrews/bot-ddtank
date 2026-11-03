package projection

import (
	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/player"
)

func projectInventory(next *game.Snapshot, f *fields) (string, error) {
	bag := f.integer("bag_type")
	if next.Player.Inventory == nil {
		next.Player.Inventory = make(map[int32]map[int32]player.Item)
	}

	if next.Player.Inventory[bag] == nil {
		next.Player.Inventory[bag] = make(map[int32]player.Item)
	}

	for _, row := range f.list("slots") {
		slot := &fields{data: row}
		place := slot.integer("place")
		if slot.flag("present") {
			data, ok := row["item"].(map[string]any)
			if !ok {
				return "", &FieldError{Name: "item"}
			}

			item := &fields{data: data}
			next.Player.Inventory[bag][place] = player.Item{
				BagType: bag, Place: place,
				TemplateID: item.integer("template_id"), Count: item.integer("count"),
				ValidDate: item.integer("valid_date"), BeginDate: item.text("begin_date"),
				Used: item.flag("used"),
			}
			if item.err != nil {
				return "", item.err
			}
		} else {
			delete(next.Player.Inventory[bag], place)
		}

		if slot.err != nil {
			return "", slot.err
		}
	}

	next.Player.InventoryRevision++
	if next.Player.BagRevisions == nil {
		next.Player.BagRevisions = make(map[int32]uint64)
	}

	next.Player.BagRevisions[bag] = next.Player.InventoryRevision

	return "inventory_changed", nil
}

func projectPlayerBuffs(next *game.Snapshot, header Header, f *fields) (string, error) {
	if next.Player.ID == nil || header.ClientID != *next.Player.ID {
		return "", nil
	}

	if header.Code == 186 {
		next.Player.BuffsKnown = true
	}

	if header.Code == 186 || next.Player.Buffs == nil {
		next.Player.Buffs = make(map[int32]player.Buff)
	}

	for _, row := range f.list("buffs") {
		entry := &fields{data: row}
		kind := entry.integer("type")
		active := entry.flag("flag")
		if active {
			next.Player.Buffs[kind] = player.Buff{
				Type: kind, Date: entry.text("date"),
				Arguments: [4]int32{entry.integer("argument_1"), entry.integer("argument_2"), entry.integer("argument_3"), entry.integer("argument_4")},
			}
		} else {
			delete(next.Player.Buffs, kind)
		}

		if entry.err != nil {
			return "", entry.err
		}
	}

	return "player_buffs_changed", nil
}
