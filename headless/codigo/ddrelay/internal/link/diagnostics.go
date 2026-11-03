package link

type Diagnostics struct {
	InputQueued     int    `json:"input_queued"`
	InputBytes      int64  `json:"input_bytes"`
	HistoryEvents   int    `json:"history_events"`
	HistoryBytes    int    `json:"history_bytes"`
	PendingRequests int    `json:"pending_requests"`
	AppliedSequence uint64 `json:"applied_sequence"`
	Disconnects     uint64 `json:"disconnects"`
}
