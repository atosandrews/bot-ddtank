package commands

type MoveStop struct {
	X, Y     int32
	MoveFlag bool
}

func (MoveStop) Name() string { return "move_stop" }
