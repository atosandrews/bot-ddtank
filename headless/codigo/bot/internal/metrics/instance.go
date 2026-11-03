package metrics

const (
	InstanceGroupingKnown InstanceGrouping = iota + 1
	InstanceGroupingUnknown
	InstanceGroupingOverflow
)

type InstanceGrouping uint8

type InstanceKey struct {
	Grouping   InstanceGrouping
	RoomType   int32
	InstanceID int32
	Difficulty int32
}

type Instance struct {
	Key InstanceKey
	Totals
}
