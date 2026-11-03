package projection

import (
	"ddbot/bot/internal/game/fight"
)

func (p *Projector) expireClientCombatState(header Header, decoded Decoded) (string, error) {
	if header.Code != 91 || decoded.Status != "decoded" {
		return "", nil
	}

	f := &fields{data: decoded.Fields}
	var destination *fight.Position
	name := "energy_unconfirmed"
	walkReport := false
	sub := f.integer("subcommand")

	switch sub {
	case 9, 10:
		destination = &fight.Position{X: f.integer("x"), Y: f.integer("y")}
		name = "position_unconfirmed"
		if sub == 9 {
			walkReport = f.integer("move_type") == 0
		}
	case 32:
	default:
		return "", nil
	}

	if f.err != nil {
		return "", f.err
	}

	next := p.session.Snapshot()
	if next.Player.ID == nil {
		return "", nil
	}

	for id, actor := range next.Fight.Actors {
		if actor.PlayerID == nil || *actor.PlayerID != *next.Player.ID {
			continue
		}

		if walkReport && unchangedPosition(actor, *destination) {
			return "", nil
		}

		actor.Energy = nil
		if destination != nil {
			actor.Position = nil
			actor.Destination = destination
		}
		next.Fight.Actors[id] = actor
		if err := p.session.Commit(next); err != nil {
			return "", err
		}

		return name, nil
	}

	return "", nil
}

func unchangedPosition(actor fight.Actor, reported fight.Position) bool {
	if actor.Position == nil {
		return false
	}

	if actor.Destination != nil {
		return false
	}

	return *actor.Position == reported
}
