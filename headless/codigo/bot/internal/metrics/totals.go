package metrics

type Totals struct {
	Fights                 Activity
	Missions               Activity
	PlayerTurns            uint64
	PlayerShotsResolved    uint64
	PlayerPropsConfirmed   uint64
	MissionExperience      uint64
	FightExperience        uint64
	AwardsReceived         uint64
	RewardItemsReceived    uint64
	Rewards                []Reward
	UngroupedAwards        uint64
	UngroupedRewardItems   uint64
	UnreadableFightResults uint64
}
