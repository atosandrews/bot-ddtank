package action

type MissionReady struct{ Ready bool }

func (MissionReady) Name() string { return "mission_ready" }
