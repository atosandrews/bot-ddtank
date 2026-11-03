package fight

import "ddbot/bot/internal/game/value"

type Actor struct {
	ID          int32
	PlayerID    *int32
	Name        *string
	Position    *Position
	Destination *Position
	HP          *int32
	PendingHP   *int32
	MaxHP       *int32
	Team        *int32
	Direction   *int32
	Alive       *bool
	Dander      *int32
	ShootCount  *int32
	Energy      *float64
	MaxEnergy   *float64
	PowerRatio  *float64
	EnergyIssue *string
	Controls    Controls
	Special     *bool
	BallID      *int32
	WeaponID    *int32
}

func (a Actor) Clone() Actor {
	a.PlayerID = value.Copy(a.PlayerID)
	a.Name = value.Copy(a.Name)
	a.Position = value.Copy(a.Position)
	a.Destination = value.Copy(a.Destination)
	a.HP = value.Copy(a.HP)
	a.PendingHP = value.Copy(a.PendingHP)
	a.MaxHP = value.Copy(a.MaxHP)
	a.Team = value.Copy(a.Team)
	a.Direction = value.Copy(a.Direction)
	a.Alive = value.Copy(a.Alive)
	a.Dander = value.Copy(a.Dander)
	a.ShootCount = value.Copy(a.ShootCount)
	a.Energy = value.Copy(a.Energy)
	a.MaxEnergy = value.Copy(a.MaxEnergy)
	a.PowerRatio = value.Copy(a.PowerRatio)
	a.EnergyIssue = value.Copy(a.EnergyIssue)
	a.Special = value.Copy(a.Special)
	a.BallID = value.Copy(a.BallID)
	a.WeaponID = value.Copy(a.WeaponID)

	return a
}
