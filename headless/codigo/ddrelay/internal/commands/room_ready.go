package commands

type RoomReady struct{ State int32 }

func (RoomReady) Name() string { return "room_ready" }
