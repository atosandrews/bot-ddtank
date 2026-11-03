package metrics

import (
	"time"

	"ddbot/bot/internal/game"
	"ddbot/bot/internal/game/fight"
)

type Observation struct {
	Sequence              uint64
	ReceivedAt            time.Time
	Server                bool
	Decoded               bool
	Name                  string
	ActorID               int32
	State                 game.Snapshot
	FightResults          []fight.ResultPlayer
	UnreadableFightResult bool
}
