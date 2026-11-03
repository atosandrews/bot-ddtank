package commands

type CardTakeExtra struct {
	Index int32
}

func (CardTakeExtra) Name() string { return "card_take_extra" }
