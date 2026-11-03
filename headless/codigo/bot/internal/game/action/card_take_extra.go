package action

type CardTakeExtra struct {
	Index int32
}

func (CardTakeExtra) Name() string { return "card_take_extra" }

func (c CardTakeExtra) CardKind() string { return CardExtra }

func (c CardTakeExtra) CardIndex() int32 { return c.Index }

func (CardTakeExtra) CompletesCards() bool { return false }
