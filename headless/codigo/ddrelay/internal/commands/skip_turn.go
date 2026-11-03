package commands

type SkipTurn struct{ Argument int32 }

func (SkipTurn) Name() string { return "skip_turn" }
