package commands

type LoadProgress struct{ Progress int32 }

func (LoadProgress) Name() string { return "load_progress" }
