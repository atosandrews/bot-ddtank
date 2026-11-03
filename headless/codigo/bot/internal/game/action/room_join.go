package action

type RoomJoin struct {
	LoginFlag                  bool
	RoomType, RoomID, Argument int32
	Password                   string
}

func (RoomJoin) Name() string { return "room_join" }
