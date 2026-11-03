package session

type pendingCommand struct {
	batch            uint64
	name             string
	scriptGeneration uint64
}
