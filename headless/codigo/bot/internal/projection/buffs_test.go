package projection

import (
	"testing"

	"ddbot/bot/internal/game"
)

func TestInventoryAndBuffState(t *testing.T) {
	session := game.NewSession(1, true)
	p := NewProjector(session)
	apply := func(code, owner int32, data map[string]any) {
		t.Helper()
		if _, err := p.Apply("ServerToClient", Header{Code: code, ClientID: owner}, Decoded{Status: "decoded", Fields: data}); err != nil {
			t.Fatal(err)
		}
	}
	apply(1, 7, map[string]any{"status": int32(0)})
	apply(64, 7, map[string]any{"bag_type": int32(3), "slots": []map[string]any{
		{"place": int32(2), "present": true, "item": map[string]any{
			"template_id": int32(10004), "count": int32(1), "valid_date": int32(0), "begin_date": "2026-10-01T00:00:00", "used": false,
		}},
	}})
	snapshot := session.Snapshot()
	item := snapshot.Player.Inventory[3][2]
	if item.TemplateID != 10004 || item.BagType != 3 || item.Place != 2 || item.Count != 1 {
		t.Fatalf("item=%+v", item)
	}

	delete(snapshot.Player.Inventory[3], 2)
	if len(session.Snapshot().Player.Inventory[3]) != 1 {
		t.Fatal("inventory alias escaped")
	}

	buff := map[string]any{"type": int32(15), "flag": true, "date": "2026-10-01T00:00:00", "argument_1": int32(0), "argument_2": int32(0), "argument_3": int32(0), "argument_4": int32(0)}
	apply(186, 8, map[string]any{"buffs": []map[string]any{buff}})
	if len(session.Snapshot().Player.Buffs) != 0 {
		t.Fatal("foreign buff applied")
	}

	apply(186, 7, map[string]any{"buffs": []map[string]any{buff}})
	if session.Snapshot().Player.Buffs[15].Type != 15 {
		t.Fatal("free prop buff absent")
	}

	snapshot = session.Snapshot()
	delete(snapshot.Player.Buffs, 15)
	if len(session.Snapshot().Player.Buffs) != 1 {
		t.Fatal("buff alias escaped")
	}

	buff["flag"] = false
	apply(185, 7, map[string]any{"buffs": []map[string]any{buff}})
	apply(64, 7, map[string]any{"bag_type": int32(3), "slots": []map[string]any{{"place": int32(2), "present": false}}})
	if len(session.Snapshot().Player.Buffs) != 0 || len(session.Snapshot().Player.Inventory[3]) != 0 {
		t.Fatal("removed item or buff retained")
	}
}

func TestEnergyAndSpecialProjectile(t *testing.T) {
	session := game.NewSession(1, true)
	p := NewProjector(session)
	for _, step := range []struct {
		fields  map[string]any
		energy  *float64
		special *bool
	}{
		{map[string]any{"subcommand": uint8(6), "turn_number": int32(1), "turn_time": int32(20), "wind_display_flag": true, "wind_display_bytes": []int8{0, 0, 0}, "livings": []map[string]any{
			{"living_id": int32(0), "x": int32(1), "y": int32(2), "hp": int32(100), "max_hp": int32(100), "max_energy": int32(240)},
		}}, ptr(float64(240)), nil},
		{map[string]any{"subcommand": uint8(20), "ball_id": int32(21), "special": true}, ptr(float64(240)), ptr(true)},
		{map[string]any{"subcommand": uint8(32)}, nil, ptr(true)},
		{map[string]any{"subcommand": uint8(41), "property": "energy2", "value": "155"}, ptr(float64(155)), ptr(true)},
	} {
		if _, err := p.Apply("ServerToClient", Header{Code: 91}, Decoded{Status: "decoded", Fields: step.fields}); err != nil {
			t.Fatal(err)
		}

		actor := session.Snapshot().Fight.Actors[0]
		if !sameOptional(actor.Energy, step.energy) || !sameOptional(actor.Special, step.special) {
			t.Fatalf("energy=%v special=%v", actor.Energy, actor.Special)
		}
	}
}

func ptr[T any](v T) *T { return &v }

func sameOptional[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}
