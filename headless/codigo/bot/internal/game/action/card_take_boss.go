package action

type CardTakeBoss struct {
	Index int32
}

func (CardTakeBoss) Name() string { return "card_take_boss" }

func (c CardTakeBoss) CardKind() string { return CardBoss }

func (c CardTakeBoss) CardIndex() int32 { return c.Index }

func (CardTakeBoss) CompletesCards() bool { return false }
