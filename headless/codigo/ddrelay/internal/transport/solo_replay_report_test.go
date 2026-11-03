package transport

type soloReplayReport struct {
	frames uint64
	shots  int
	props  int
	audit  map[string]any
}
