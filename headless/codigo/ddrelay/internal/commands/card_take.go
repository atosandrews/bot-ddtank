package commands

type CardTake struct {
	Index int32
}

func (CardTake) Name() string { return "card_take" }
