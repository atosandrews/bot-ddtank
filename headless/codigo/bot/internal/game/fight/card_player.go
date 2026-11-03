package fight

type CardPlayer struct {
	PlayerID       int32
	Level          int32
	Treatment      int32
	GainedGP       int32
	Winner         bool
	CardCount      int32
	ResultArgument int32
	HasLevelAgain  bool
	HasCardGet     bool
}
