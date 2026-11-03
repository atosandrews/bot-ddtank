package action

type MoveStart struct {
	Moving     bool
	MoveType   int8
	X, Y       int32
	Direction  int8
	Living     bool
	TurnNumber int16
}

func (MoveStart) Name() string { return "move_start" }
