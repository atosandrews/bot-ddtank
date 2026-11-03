package fight

import "ddbot/bot/internal/game/value"

type State struct {
	Generation   uint64
	TurnRevision uint64
	Phase        string
	RoomType     *int32
	MapID        *int32
	MissionID    *int32
	Turn         *Turn
	Wind         Wind
	Actors       map[int32]Actor
	Cards        Cards
}

func NewState() State { return State{Phase: "unknown", Actors: make(map[int32]Actor)} }

func (s State) Clone() State {
	s.RoomType = value.Copy(s.RoomType)
	s.MapID = value.Copy(s.MapID)
	s.MissionID = value.Copy(s.MissionID)
	s.Turn = value.Copy(s.Turn)
	s.Wind = s.Wind.Clone()
	s.Cards = s.Cards.Clone()
	actors := make(map[int32]Actor, len(s.Actors))

	for id, actor := range s.Actors {
		actors[id] = actor.Clone()
	}

	s.Actors = actors

	return s
}
