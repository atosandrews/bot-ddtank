package projection

import (
	"math"

	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/fight"
	"ddbot/bot/internal/game/value"
)

const activityDungeonRoomType int32 = 21

func projectFight(next *game.Snapshot, header Header, f *fields) (string, error) {
	sub := f.integer("subcommand")
	state := &next.Fight

	switch sub {
	case 101:
		generation := state.Generation + 1
		*state = fight.NewState()
		state.Generation = generation
		state.Phase = "created"
		if _, exists := f.data["room_type"]; exists {
			state.RoomType = value.Of(f.integer("room_type"))
		}
		for _, row := range f.list("players") {
			actorFields := &fields{data: row}
			id := actorFields.integer("living_id")
			state.Actors[id] = fight.Actor{
				ID:       id,
				PlayerID: value.Of(actorFields.integer("player_id")),
				Name:     value.Of(actorFields.text("nickname")),
				Team:     value.Of(actorFields.integer("team")),
				MaxHP:    value.Of(actorFields.integer("max_hp")),
			}
			if _, ok := row["weapon_id"]; ok {
				actor := state.Actors[id]
				actor.WeaponID = value.Of(actorFields.integer("weapon_id"))
				state.Actors[id] = actor
			}

			if actorFields.err != nil {
				f.err = actorFields.err
			}
		}
		return "fight_created", nil
	case 103:
		players := make(map[int32]fight.Actor)
		for id, actor := range state.Actors {
			if actor.PlayerID != nil {
				players[id] = fight.Actor{
					ID:       id,
					PlayerID: actor.PlayerID,
					Name:     actor.Name,
					Team:     actor.Team,
					WeaponID: actor.WeaponID,
				}
			}
		}

		state.Actors = players
		state.Turn = nil
		state.TurnRevision++
		state.Wind = fight.Wind{}
		state.Cards = state.Cards.Reset()
		state.MapID = value.Of(f.integer("map_id"))
		state.Phase = "loading"
		return "fight_loading", nil
	case 46:
		for id, actor := range state.Actors {
			if next.Player.ID == nil || actor.PlayerID == nil || *actor.PlayerID != *next.Player.ID {
				continue
			}
			if (state.RoomType == nil || *state.RoomType == activityDungeonRoomType) && header.Parameter1 != actor.ID {
				continue
			}
			actor.ShootCount = value.Of(f.integer("shoot_count"))
			state.Actors[id] = actor
		}
		return "shoot_count_changed", nil
	case 99:
		for _, row := range f.list("players") {
			projectActor(state, row, f)
		}
		state.Phase = "active"
		return "fight_started", nil
	case 64:
		projectActor(state, f.data, f)
		return "actor_spawned", nil
	case 6:
		state.TurnRevision++
		if actor, ok := state.Actors[header.Parameter1]; ok {
			actor.Energy = nil
			actor.ShootCount = nil
			state.Actors[actor.ID] = actor
		}
		for _, row := range f.list("livings") {
			projectActor(state, row, f)
		}
		if actor, ok := state.Actors[header.Parameter1]; ok {
			actor.BallID = nil
			actor.Special = nil
			state.Actors[actor.ID] = actor
		}
		state.Turn = &fight.Turn{ActorID: header.Parameter1, Number: f.integer("turn_number"), Time: f.integer("turn_time")}
		state.Wind.Expire()
		state.Wind.PictureID = nil
		state.Wind.PictureSHA256 = nil
		projectWindDisplay(state, f)
		resolveWindDisplay(state)
		state.Phase = "active"
		return "turn_started", nil
	case 2:
		state.Wind.Tenths = value.Of(f.integer("wind_tenths"))
		state.Wind.Current = true
		state.Wind.Source = value.Of(fight.WindSourceFire)
		projectWindDisplay(state, f)
		projectShot(state, f)
		return "shot_resolved", nil
	case 55, 56:
		id := header.Parameter1
		actor := state.Actors[id]
		actor.ID = id
		if sub == 55 {
			actor.Position = &fight.Position{X: f.integer("from_x"), Y: f.integer("from_y")}
			actor.Destination = &fight.Position{X: f.integer("to_x"), Y: f.integer("to_y")}
		} else {
			actor.Destination = &fight.Position{X: f.integer("x"), Y: f.integer("y")}
			actor.Position = nil
		}
		state.Actors[id] = actor
		return "actor_moved", nil
	case 11:
		actor := state.Actors[header.Parameter1]
		actor.ID = header.Parameter1
		hp := f.integer("hp")
		if f.integer("change_type") == 7 {
			if actor.HP == nil {
				return "health_changed", nil
			}

			adjusted := int64(*actor.HP) - int64(f.integer("effect_amount"))
			if adjusted < math.MinInt32 || adjusted > math.MaxInt32 {
				return "", &FieldError{Name: "effect_amount overflow"}
			}

			hp = int32(adjusted)
		}
		actor.HP = value.Of(hp)
		actor.PendingHP = nil
		state.Actors[actor.ID] = actor
		return "health_changed", nil
	case 58:
		for _, row := range f.list("targets") {
			target := &fields{data: row}
			id := target.integer("target_id")
			actor := state.Actors[id]
			actor.ID = id
			actor.PendingHP = value.Of(target.integer("hp"))
			state.Actors[id] = actor
			if target.err != nil {
				f.err = target.err
			}
		}
		return "damage_resolved", nil
	case 7, 14, 20:
		actor := state.Actors[header.Parameter1]
		actor.ID = header.Parameter1
		if sub == 7 {
			actor.Direction = value.Of(f.integer("direction"))
		}
		if sub == 14 {
			actor.Dander = value.Of(f.integer("dander"))
		}
		if sub == 20 {
			actor.BallID = value.Of(f.integer("ball_id"))
			if _, ok := f.data["special"]; ok {
				actor.Special = value.Of(f.flag("special"))
			}
		}
		state.Actors[actor.ID] = actor
		return "actor_changed", nil
	case 32:
		actor := state.Actors[header.Parameter1]
		actor.ID = header.Parameter1
		actor.Energy = nil
		state.Actors[actor.ID] = actor
		return "prop_used", nil
	case 41:
		actor := state.Actors[header.Parameter1]
		actor.ID = header.Parameter1
		projectFightProperty(&actor, f.text("property"), f.text("value"))
		state.Actors[actor.ID] = actor
		return "actor_property_changed", nil
	case 113:
		state.MissionID = value.Of(f.integer("mission_id"))
		return "mission_changed", nil
	case 98, 89:
		return projectCards(next, f)
	case 112, 115:
		if sub == 112 {
			projectCardRound(state, f)
		}
		state.Phase = "mission_ended"
		if sub == 115 {
			state.Phase = "ended"
		}
		state.Turn = nil
		state.Wind.Expire()
		return state.Phase, nil
	case 241:
		pictureID := f.integer("picture_id")
		if pictureID < math.MinInt8 || pictureID > math.MaxInt8 {
			return "", &FieldError{Name: "picture_id"}
		}
		id := int8(pictureID)
		state.Wind.PictureID = &id
		state.Wind.PictureSHA256 = value.Of(f.text("picture_sha256"))
		if state.Wind.Pictures == nil {
			state.Wind.Pictures = make(map[int8]fight.WindPicture)
		}
		glyph := ""
		if _, exists := f.data["picture_glyph"]; exists {
			glyph = f.text("picture_glyph")
		}
		state.Wind.Pictures[id] = fight.WindPicture{SHA256: *state.Wind.PictureSHA256, Glyph: glyph}
		if state.Turn != nil && (state.Wind.Source == nil || *state.Wind.Source != fight.WindSourceFire) {
			resolveWindDisplay(state)
		}
		return "wind_display_changed", nil
	default:
		return "", &UnsupportedProjectionError{}
	}
}

func projectActor(state *fight.State, row map[string]any, parent *fields) {
	f := &fields{data: row}
	id := f.integer("living_id")
	actor := state.Actors[id]
	actor.ID = id
	actor.Position = &fight.Position{X: f.integer("x"), Y: f.integer("y")}
	actor.Destination = nil
	actor.HP = value.Of(f.integer("hp"))
	actor.PendingHP = nil
	actor.MaxHP = value.Of(f.integer("max_hp"))
	if _, ok := row["name"]; ok {
		actor.Name = value.Of(f.text("name"))
	}

	if _, ok := row["team"]; ok {
		actor.Team = value.Of(f.integer("team"))
	}

	if _, ok := row["direction"]; ok {
		actor.Direction = value.Of(f.integer("direction"))
	}

	if _, ok := row["alive"]; ok {
		actor.Alive = value.Of(f.flag("alive"))
	}

	if _, ok := row["dander"]; ok {
		actor.Dander = value.Of(f.integer("dander"))
	}

	if _, ok := row["shoot_count"]; ok {
		actor.ShootCount = value.Of(f.integer("shoot_count"))
	}

	if _, ok := row["max_energy"]; ok {
		actor.MaxEnergy = value.Of(float64(f.integer("max_energy")))
		actor.Energy = value.Copy(actor.MaxEnergy)
		actor.EnergyIssue = nil
	}

	if _, ok := row["power_ratio"]; ok {
		actor.PowerRatio = value.Of(float64(f.integer("power_ratio")) / 100)
	}

	if _, ok := row["properties"]; ok {
		actor.Controls = fight.Controls{Known: true, RightPropEnabled: true}
		if _, ok := row["frozen"]; ok {
			actor.Controls.Frozen = f.flag("frozen")
		}

		for _, property := range f.list("properties") {
			entry := &fields{data: property}
			projectFightProperty(&actor, entry.text("name"), entry.text("value"))
			if entry.err != nil {
				f.err = entry.err
			}
		}
	}

	state.Actors[id] = actor
	if f.err != nil {
		parent.err = f.err
	}
}

func projectWindDisplay(state *fight.State, f *fields) {
	direction := int32(-1)
	if f.flag("wind_display_flag") {
		direction = 1
	}

	state.Wind.Direction = &direction

	display := f.signedBytes("wind_display_bytes")
	if f.err != nil || len(display) != 3 {
		f.err = &FieldError{Name: "wind_display_bytes"}

		return
	}

	state.Wind.Display = &[3]int8{display[0], display[1], display[2]}
}

func projectShot(state *fight.State, f *fields) {
	for _, projectile := range f.list("projectiles") {
		p := &fields{data: projectile}

		for _, row := range p.list("actions") {
			a := &fields{data: row}
			kind := a.integer("type")
			if a.err != nil {
				f.err = a.err

				return
			}

			if kind != 5 && kind != 11 {
				continue
			}

			id := a.integer("target_id")
			actor := state.Actors[id]
			actor.ID = id
			actor.PendingHP = value.Of(a.integer("hp"))
			state.Actors[id] = actor
			if a.err != nil {
				f.err = a.err
			}
		}

		if p.err != nil {
			f.err = p.err
		}
	}

	for _, row := range f.list("pet_targets") {
		target := &fields{data: row}
		id := target.integer("target_id")
		actor := state.Actors[id]
		actor.ID = id
		actor.PendingHP = value.Of(target.integer("hp"))
		state.Actors[id] = actor
		if target.err != nil {
			f.err = target.err
		}
	}
}
