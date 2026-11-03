package script

import (
	"cmp"
	"slices"
	"strconv"

	"ddbot/bot/internal/game/player"
)

func (r *Runtime) bagRevisions() map[string]uint64 {
	revisions := make(map[string]uint64)
	for bag, revision := range r.session.Snapshot().Player.BagRevisions {
		revisions[strconv.FormatInt(int64(bag), 10)] = revision
	}

	return revisions
}

func (r *Runtime) bagLock() map[string]any {
	state := r.session.Snapshot().Player.BagLock
	if state == nil {
		return map[string]any{"known": false}
	}

	return map[string]any{
		"known": true, "revision": state.Revision, "operation": state.Operation,
		"success": state.Success, "message": state.Message,
	}
}

func (r *Runtime) inventory() []map[string]any {
	items := make([]player.Item, 0)
	for _, slots := range r.session.Snapshot().Player.Inventory {
		for _, item := range slots {
			items = append(items, item)
		}
	}

	slices.SortFunc(items, func(a, b player.Item) int {
		if a.BagType != b.BagType {
			return cmp.Compare(a.BagType, b.BagType)
		}

		return cmp.Compare(a.Place, b.Place)
	})

	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, map[string]any{
			"bag_type": item.BagType, "place": item.Place, "template_id": item.TemplateID,
			"count": item.Count, "valid_date": item.ValidDate, "begin_date": item.BeginDate, "used": item.Used,
		})
	}

	return rows
}

func (r *Runtime) playerBuffs() []map[string]any {
	buffs := make([]player.Buff, 0)
	for _, buff := range r.session.Snapshot().Player.Buffs {
		buffs = append(buffs, buff)
	}

	slices.SortFunc(buffs, func(a, b player.Buff) int { return cmp.Compare(a.Type, b.Type) })

	rows := make([]map[string]any, 0, len(buffs))
	for _, buff := range buffs {
		rows = append(rows, map[string]any{
			"type": buff.Type, "date": buff.Date, "arguments": buff.Arguments[:],
		})
	}

	return rows
}
