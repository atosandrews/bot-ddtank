package commands

type RoomSetup struct {
	MapID                int32
	RoomType             int32
	Password, NameValue  string
	TimeType, Difficulty int32
	LevelLimits          int32
	Flag1, Flag2         bool
}

func (RoomSetup) Name() string { return "room_setup" }
