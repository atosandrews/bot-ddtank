package social

type Invitation struct {
	PlayerID      int32
	RoomID        int32
	MapID         int32
	SecondType    int32
	GameMode      int32
	Difficulty    int32
	LevelLimits   int32
	Name          string
	RoomName      string
	VIP           bool
	VIPLevel      int32
	BarrierNumber int32
}
