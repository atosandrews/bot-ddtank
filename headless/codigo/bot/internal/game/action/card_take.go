package action

type CardTake struct {
	Index int32
}

func (CardTake) Name() string { return "card_take" }

func (c CardTake) CardKind() string { return CardNormal }

func (c CardTake) CardIndex() int32 { return c.Index }

func (CardTake) CompletesCards() bool { return false }
