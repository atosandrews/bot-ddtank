package action

type Request struct {
	ID              uint64
	Command         Command
	FightGeneration uint64
	TurnRevision    uint64
	TurnNumber      *int32
	CardRevision    uint64
	RoomID          *int32
}
