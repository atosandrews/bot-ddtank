package room

import "ddbot/bot/internal/game/value"

type Player struct {
	ID    int32
	Name  *string
	Team  *int32
	Place *int32
}

func (p Player) Clone() Player {
	p.Name = value.Copy(p.Name)
	p.Team = value.Copy(p.Team)
	p.Place = value.Copy(p.Place)

	return p
}
