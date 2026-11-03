package script

import (
	"slices"

	"ddbot/bot/internal/game/fight"
)

func actorFields(a fight.Actor) map[string]any {
	return map[string]any{
		"id":           a.ID,
		"player_id":    optional(a.PlayerID),
		"name":         optional(a.Name),
		"position":     positionFields(a.Position),
		"destination":  positionFields(a.Destination),
		"hp":           optional(a.HP),
		"pending_hp":   optional(a.PendingHP),
		"max_hp":       optional(a.MaxHP),
		"team":         optional(a.Team),
		"direction":    optional(a.Direction),
		"alive":        optional(a.Alive),
		"dander":       optional(a.Dander),
		"shoot_count":  optional(a.ShootCount),
		"energy":       optional(a.Energy),
		"max_energy":   optional(a.MaxEnergy),
		"power_ratio":  optional(a.PowerRatio),
		"energy_issue": optional(a.EnergyIssue),
		"controls": map[string]any{
			"known": a.Controls.Known, "frozen": a.Controls.Frozen,
			"lock_state": a.Controls.LockState, "lock_type": a.Controls.LockType,
			"right_prop_enabled": a.Controls.RightPropEnabled,
			"lock_right_prop":    a.Controls.LockRightProp, "lock_spell_kill": a.Controls.LockSpellKill,
		},
		"special":   optional(a.Special),
		"ball_id":   optional(a.BallID),
		"weapon_id": optional(a.WeaponID),
	}
}

func positionFields(p *fight.Position) any {
	if p == nil {
		return nil
	}

	return map[string]any{"x": p.X, "y": p.Y}
}

func turnFields(t *fight.Turn) any {
	if t == nil {
		return nil
	}

	return map[string]any{"actor_id": t.ActorID, "number": t.Number, "time": t.Time}
}

func (r *Runtime) actors() []map[string]any {
	actors := r.session.Snapshot().Fight.Actors
	ids := make([]int32, 0, len(actors))

	for id := range actors {
		ids = append(ids, id)
	}

	slices.Sort(ids)
	rows := make([]map[string]any, 0, len(ids))

	for _, id := range ids {
		rows = append(rows, actorFields(actors[id]))
	}

	return rows
}

func (r *Runtime) fightState() map[string]any {
	f := r.session.Snapshot().Fight

	return map[string]any{
		"generation":    f.Generation,
		"turn_revision": f.TurnRevision,
		"phase":         f.Phase,
		"map_id":        optional(f.MapID),
		"mission_id":    optional(f.MissionID),
		"turn":          turnFields(f.Turn),
		"wind":          windFields(f.Wind),
		"actors":        r.actors(),
		"cards":         r.cardState(),
	}
}

func (r *Runtime) roomState() map[string]any {
	room := r.session.Snapshot().Room
	ids := make([]int32, 0, len(room.Players))

	for id := range room.Players {
		ids = append(ids, id)
	}

	slices.Sort(ids)
	rows := make([]map[string]any, 0, len(ids))

	for _, id := range ids {
		p := room.Players[id]
		rows = append(rows, map[string]any{
			"id":    p.ID,
			"name":  optional(p.Name),
			"team":  optional(p.Team),
			"place": optional(p.Place),
		})
	}

	return map[string]any{
		"id":                       optional(room.ID),
		"map_id":                   optional(room.MapID),
		"name":                     optional(room.Name),
		"difficulty":               optional(room.Difficulty),
		"started":                  optional(room.Started),
		"last_join_success":        optional(room.LastJoinSuccess),
		"start_cancelled_revision": room.StartCancelledRevision,
		"players":                  rows,
	}
}
