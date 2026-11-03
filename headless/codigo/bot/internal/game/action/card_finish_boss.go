package action

type CardFinishBoss struct {
}

func (CardFinishBoss) Name() string { return "card_finish_boss" }

func (c CardFinishBoss) CardKind() string { return CardBoss }

func (c CardFinishBoss) CardIndex() int32 { return 0 }

func (CardFinishBoss) CompletesCards() bool { return true }
