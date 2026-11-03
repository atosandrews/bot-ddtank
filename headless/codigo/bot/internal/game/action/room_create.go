package action

type RoomCreate struct {
	RoomType, TimeType  int8
	NameValue, Password string
}

func (RoomCreate) Name() string { return "room_create" }
