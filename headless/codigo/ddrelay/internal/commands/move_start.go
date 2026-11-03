package commands

type MoveStart struct {
	Moving     bool
	MoveType   int32
	X, Y       int32
	Direction  int32
	Living     bool
	TurnNumber int32
}

func (MoveStart) Name() string { return "move_start" }
