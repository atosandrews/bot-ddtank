package action

const CardNormal = "normal"
const CardBoss = "boss"
const CardExtra = "extra"

type CardCommand interface {
	Command
	CardKind() string
	CardIndex() int32
	CompletesCards() bool
}
