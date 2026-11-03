package session

import (
	"context"
	"time"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/projection"
)

func (s *Session) packet(ctx context.Context, event *pb.PacketEvent) ([]*pb.BotMessage, error) {
	if event.Sequence <= s.sequence {
		return []*pb.BotMessage{{Payload: &pb.BotMessage_Applied{Applied: &pb.EventsApplied{Sequence: s.sequence}}}}, nil
	}

	if event.Sequence != s.sequence+1 {
		s.state.Invalidate("event sequence gap")

		return nil, &Error{Code: "resync_required", Reason: "event sequence gap"}
	}

	fields, err := contract.DecodeFields(event.Fields)
	if err != nil {
		return nil, err
	}

	direction, err := contract.PacketDirectionName(event.Direction)
	if err != nil {
		return nil, err
	}

	origin, err := contract.PacketOriginName(event.Origin)
	if err != nil {
		return nil, err
	}

	status, err := contract.DecodeStatusName(event.Status)
	if err != nil {
		return nil, err
	}

	compression, err := contract.CompressionName(event.Compression)
	if err != nil {
		return nil, err
	}

	h := event.Header
	name, err := s.projector.Apply(direction, projection.Header{
		Code:       h.Code,
		ClientID:   h.ClientId,
		Parameter1: h.Parameter1,
		Parameter2: h.Parameter2,
	}, projection.Decoded{Status: status, Fields: fields})
	if err != nil {
		s.state.Invalidate("projection failed")

		return nil, err
	}

	if event.Direction == pb.PacketDirection_PACKET_DIRECTION_SERVER_TO_CLIENT && h.Code == 1 && name == "player_loaded" {
		s.initialized = true
		s.state.SetWritable(true)
	}

	snapshot := s.state.Snapshot()
	s.observeMetrics(event, name, fields, snapshot)

	packet := map[string]any{
		"stream":      snapshot.Stream,
		"time":        time.Unix(0, event.UnixNano).UTC().Format(time.RFC3339Nano),
		"direction":   direction,
		"origin":      origin,
		"command_id":  event.CommandId,
		"code":        h.Code,
		"client_id":   h.ClientId,
		"parameter1":  h.Parameter1,
		"parameter2":  h.Parameter2,
		"name":        event.Name,
		"status":      status,
		"fields":      fields,
		"consumed":    event.Consumed,
		"total":       event.Total,
		"remaining":   event.Remaining,
		"compression": compression,
	}
	if err := s.runtime.Packet(ctx, packet); err != nil {
		s.state.Invalidate("callback failed")

		return nil, err
	}

	if name != "" {
		if err := s.runtime.Event(ctx, name, fields); err != nil {
			s.state.Invalidate("callback failed")

			return nil, err
		}
	}

	s.sequence = event.Sequence
	messages, err := s.collect()
	if err != nil {
		return nil, err
	}

	messages = append(messages, &pb.BotMessage{Payload: &pb.BotMessage_Applied{Applied: &pb.EventsApplied{Sequence: s.sequence}}})

	return messages, nil
}
