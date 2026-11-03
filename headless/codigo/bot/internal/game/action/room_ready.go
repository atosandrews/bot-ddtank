package action

type RoomReady struct{ State int8 }

func (RoomReady) Name() string { return "room_ready" }
