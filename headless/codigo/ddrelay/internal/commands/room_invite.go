package commands

type RoomInvite struct {
	PlayerID int32
}

func (RoomInvite) Name() string { return "room_invite" }
