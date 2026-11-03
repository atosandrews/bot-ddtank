package action

type RoomSetup struct {
	MapID                int32
	RoomType             int8
	Password, NameValue  string
	TimeType, Difficulty int8
	LevelLimits          int32
	Flag1, Flag2         bool
}

func (RoomSetup) Name() string { return "room_setup" }
