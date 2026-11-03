package transport

import "ddbot/relay/internal/link"

type controlReply struct {
	info       Info
	checkpoint link.Checkpoint
	written    int
	err        error
}
