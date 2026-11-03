package commands

type RoomCreate struct {
	RoomType, TimeType  int32
	NameValue, Password string
}

func (RoomCreate) Name() string { return "room_create" }
