package session

import (
	"time"

	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/game"
	"ddbot/bot/internal/metrics"
	"ddbot/bot/internal/projection"
)

func (s *Session) observeMetrics(event *pb.PacketEvent, name string, fields map[string]any, snapshot game.Snapshot) {
	observation := metrics.Observation{
		Sequence:   event.Sequence,
		ReceivedAt: time.Now(),
		Server:     event.Direction == pb.PacketDirection_PACKET_DIRECTION_SERVER_TO_CLIENT,
		Decoded:    event.Status == pb.DecodeStatus_DECODE_STATUS_DECODED,
		Name:       name,
		ActorID:    event.Header.Parameter1,
		State:      snapshot,
	}
	if name == "ended" {
		results, err := projection.ReadFightResults(fields)
		observation.FightResults = results
		observation.UnreadableFightResult = err != nil
	}

	s.metrics.Observe(observation)
}

func (r *Registry) Metrics(tenant, id string) (metrics.Snapshot, bool, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return metrics.Snapshot{}, false, &Error{Code: "service_closed"}
	}

	for key, entry := range r.entries {
		if key.tenant != tenant || entry.id != id || entry.session == nil {
			continue
		}

		recorder := entry.session.metrics
		attached := entry.attached
		r.mu.Unlock()

		return recorder.Snapshot(), attached, nil
	}

	r.mu.Unlock()

	return metrics.Snapshot{}, false, &Error{Code: "session_not_found"}
}
