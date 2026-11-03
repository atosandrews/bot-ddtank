package commands

type Direction struct{ Direction int32 }

func (Direction) Name() string { return "direction" }
