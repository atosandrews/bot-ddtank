package player

type BagLock struct {
	Revision  uint64
	Operation int32
	Success   bool
	Message   string
}
