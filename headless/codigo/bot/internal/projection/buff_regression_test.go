package projection

import (
	"strconv"
	"strings"
	"testing"

	"ddbot/bot/internal/game"
)

func TestPartialAvailabilityClearsOldValues(t *testing.T) {
	for _, code := range []int32{64, 185, 186} {
		t.Run(strconv.Itoa(int(code)), func(t *testing.T) {
			session := game.NewSession(1, true)
			p := NewProjector(session)
			apply := func(code int32, status string, data map[string]any) {
				t.Helper()
				if _, err := p.Apply("ServerToClient", Header{Code: code, ClientID: 7}, Decoded{Status: status, Fields: data}); err != nil {
					t.Fatal(err)
				}
			}
			apply(1, "decoded", map[string]any{"status": int32(0)})
			apply(64, "decoded", map[string]any{"bag_type": int32(3), "slots": []map[string]any{{"place": int32(2), "present": true, "item": map[string]any{"template_id": int32(10004), "count": int32(1), "valid_date": int32(0), "begin_date": "2026-10-01T00:00:00", "used": false}}}})
			apply(186, "decoded", map[string]any{"buffs": []map[string]any{{"type": int32(15), "flag": true, "date": "2026-10-01T00:00:00", "argument_1": int32(0), "argument_2": int32(0), "argument_3": int32(0), "argument_4": int32(0)}}})
			apply(code, "partial", map[string]any{})
			s := session.Snapshot().Player
			if code == 64 && (len(s.Inventory) != 0 || !s.InventoryUncertain) {
				t.Fatal("stale inventory survived incomplete update")
			}
			if code != 64 && (len(s.Buffs) != 0 || s.BuffsKnown) {
				t.Fatal("stale buffs survived incomplete update")
			}
		})
	}
}

func TestEnergyNumericFormats(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  float64
		known bool
	}{
		{"155.5", 155.5, true}, {" 1.555e2 ", 155.5, true}, {"0xA", 10, true}, {"", 0, true},
		{"1_0", 0, false}, {"garbage", 0, false}, {"NaN", 0, false}, {"Infinity", 0, false}, {"1e1000", 0, false},
	} {
		t.Run(tt.input, func(t *testing.T) {
			s := game.NewSession(1, true)
			p := NewProjector(s)
			_, err := p.Apply("ServerToClient", Header{Code: 91}, Decoded{Status: "decoded", Fields: map[string]any{"subcommand": uint8(41), "property": "energy2", "value": tt.input}})
			if err != nil {
				t.Fatal(err)
			}
			energy := s.Snapshot().Fight.Actors[0].Energy
			if (energy != nil) != tt.known {
				t.Fatalf("energy=%v known=%v", energy, tt.known)
			}
			if tt.known && float64(*energy) != tt.want {
				t.Fatalf("energy=%v want=%v", *energy, tt.want)
			}
		})
	}
}

func TestFightControlsFollowOrderedProperties(t *testing.T) {
	session := game.NewSession(1, true)
	p := NewProjector(session)
	apply := func(status string, fields map[string]any) {
		t.Helper()
		if _, err := p.Apply("ServerToClient", Header{Code: 91}, Decoded{Status: status, Fields: fields}); err != nil {
			t.Fatal(err)
		}
	}
	apply("decoded", map[string]any{"subcommand": uint8(101), "players": []map[string]any{{"living_id": int32(0), "player_id": int32(7), "nickname": "Player", "team": int32(1), "max_hp": int32(1000)}}})
	apply("decoded", map[string]any{"subcommand": uint8(99), "players": []map[string]any{{
		"living_id": int32(0), "x": int32(680), "y": int32(504), "hp": int32(1000), "max_hp": int32(1000), "frozen": false,
		"properties": []map[string]any{{"name": "system", "value": "true"}, {"name": "silenced", "value": "false"}},
	}}})
	controls := session.Snapshot().Fight.Actors[0].Controls
	if !controls.Known || !controls.LockSpellKill || controls.LockState || !controls.RightPropEnabled || controls.LockType != 1 {
		t.Fatalf("ordered properties: %+v", controls)
	}

	apply("decoded", map[string]any{"subcommand": uint8(41), "property": "silencedSpecial", "value": "False"})
	controls = session.Snapshot().Fight.Actors[0].Controls
	if controls.LockSpellKill || controls.LockState || !controls.RightPropEnabled || controls.LockType != 3 {
		t.Fatalf("unlock special: %+v", controls)
	}

	apply("decoded", map[string]any{"subcommand": uint8(41), "property": "silenceMany", "value": "1"})
	if !session.Snapshot().Fight.Actors[0].Controls.LockRightProp {
		t.Fatal("nonempty client boolean was ignored")
	}

	apply("decoded", map[string]any{"subcommand": uint8(6), "turn_number": int32(2), "turn_time": int32(30), "wind_display_flag": true, "wind_display_bytes": []int8{0, 0, 0}, "livings": []map[string]any{{"living_id": int32(0), "x": int32(680), "y": int32(504), "hp": int32(1000), "max_hp": int32(1000), "max_energy": int32(240)}}})
	if !session.Snapshot().Fight.Actors[0].Controls.LockRightProp {
		t.Fatal("turn reset discarded prop lock")
	}

	apply("partial", map[string]any{"subcommand": uint8(41)})
	if session.Snapshot().Fight.Actors[0].Controls.Known {
		t.Fatal("partial property left controls known")
	}
	apply("decoded", map[string]any{"subcommand": uint8(103), "map_id": int32(1121)})
	actor := session.Snapshot().Fight.Actors[0]
	if actor.PlayerID == nil || actor.Controls.Known || actor.Controls.LockRightProp || actor.Energy != nil {
		t.Fatal("mission load retained old control state or lost the player")
	}
}

func TestEnergyBoundsAndRecovery(t *testing.T) {
	session := game.NewSession(1, true)
	p := NewProjector(session)
	for _, step := range []struct {
		property, raw   string
		energy, maximum *float64
		issue           bool
	}{
		{"energy", "240.5", ptr(240.5), ptr(240.5), false},
		{"energy2", "155.5", ptr(155.5), ptr(240.5), false},
		{"energy2", "241", ptr(155.5), ptr(240.5), false},
		{"energy2", "-1", ptr(0.0), ptr(240.5), false},
		{"energy2", "bad", nil, ptr(240.5), true},
		{"energy2", "100.25", ptr(100.25), ptr(240.5), false},
		{"energy", "Infinity", nil, nil, true},
	} {
		if _, err := p.Apply("ServerToClient", Header{Code: 91}, Decoded{Status: "decoded", Fields: map[string]any{
			"subcommand": uint8(41), "property": step.property, "value": step.raw,
		}}); err != nil {
			t.Fatal(err)
		}

		actor := session.Snapshot().Fight.Actors[0]
		if !sameOptional(actor.Energy, step.energy) || !sameOptional(actor.MaxEnergy, step.maximum) || (actor.EnergyIssue != nil) != step.issue {
			t.Fatalf("property %s=%s: %+v", step.property, step.raw, actor)
		}
		if actor.EnergyIssue != nil && !strings.Contains(*actor.EnergyIssue, step.property+" property:") {
			t.Fatalf("missing error context: %s", *actor.EnergyIssue)
		}
	}
}
