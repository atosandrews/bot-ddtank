package action

type CardFinish struct {
}

func (CardFinish) Name() string { return "card_finish" }

func (c CardFinish) CardKind() string { return CardNormal }

func (c CardFinish) CardIndex() int32 { return 0 }

func (CardFinish) CompletesCards() bool { return true }
