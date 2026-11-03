package commands

type MissionStart struct{ Flag bool }

func (MissionStart) Name() string { return "mission_start" }
