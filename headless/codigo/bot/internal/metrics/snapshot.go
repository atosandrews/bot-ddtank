package metrics

import "time"

type Snapshot struct {
	StartedAt         time.Time
	ObservedAt        time.Time
	ObservedSequence  uint64
	ServerPackets     uint64
	ClientPackets     uint64
	IncompletePackets uint64
	Totals
	Instances []Instance
}
