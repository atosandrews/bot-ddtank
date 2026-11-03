package link

import (
	"google.golang.org/protobuf/proto"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
)

type eventHistory struct {
	events  []*pb.PacketEvent
	bytes   int
	applied uint64
	last    uint64
}

func (h *eventHistory) append(event *pb.PacketEvent) error {
	if event.Sequence != h.last+1 {
		return &Error{Code: "resync_required", Reason: "local event sequence gap"}
	}

	size := proto.Size(event)
	if len(h.events) >= contract.MaxEvents || h.bytes+size > contract.MaxHistoryBytes {
		return &Error{Code: "resync_required", Reason: "event history exceeds reconnect limits"}
	}

	h.last = event.Sequence
	h.bytes += size
	h.events = append(h.events, event)

	return nil
}

func (h *eventHistory) acknowledge(sequence uint64) error {
	if sequence < h.applied || sequence > h.last {
		return &Error{Code: "invalid_ack", Reason: "acknowledgement outside retained event range"}
	}

	h.applied = sequence

	for len(h.events) > 0 && h.events[0].Sequence <= sequence {
		h.bytes -= proto.Size(h.events[0])
		h.events[0] = nil
		h.events = h.events[1:]
	}

	return nil
}
