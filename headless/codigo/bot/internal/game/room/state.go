package room

import "ddbot/bot/internal/game/value"

type State struct {
	StartCancelledRevision uint64
	LastJoinSuccess        *bool
	ID                     *int32
	MapID                  *int32
	Name                   *string
	Difficulty             *int32
	Started                *bool
	Players                map[int32]Player
}

func NewState() State { return State{Players: make(map[int32]Player)} }

func (s State) Clone() State {
	s.ID = value.Copy(s.ID)
	s.MapID = value.Copy(s.MapID)
	s.Name = value.Copy(s.Name)
	s.Difficulty = value.Copy(s.Difficulty)
	s.Started = value.Copy(s.Started)
	s.LastJoinSuccess = value.Copy(s.LastJoinSuccess)
	players := make(map[int32]Player, len(s.Players))

	for id, player := range s.Players {
		players[id] = player.Clone()
	}

	s.Players = players

	return s
}
