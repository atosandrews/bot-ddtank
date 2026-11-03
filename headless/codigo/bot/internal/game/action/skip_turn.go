package action

type SkipTurn struct{ Argument int8 }

func (SkipTurn) Name() string { return "skip_turn" }
