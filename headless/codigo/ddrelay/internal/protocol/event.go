package protocol

import (
	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/packets"
)

func Event(sequence uint64, record capture.Record, header framing.Header, decoded packets.Result) (*pb.PacketEvent, error) {
	fields, err := contract.EncodeFields(decoded.Fields)
	if err != nil {
		return nil, err
	}

	direction, err := contract.ParsePacketDirection(string(record.Direction))
	if err != nil {
		return nil, err
	}

	origin, err := contract.ParsePacketOrigin(record.Origin)
	if err != nil {
		return nil, err
	}

	status, err := contract.ParseDecodeStatus(decoded.Status)
	if err != nil {
		return nil, err
	}

	compression, err := contract.ParseCompression(decoded.Compression)
	if err != nil {
		return nil, err
	}

	event := &pb.PacketEvent{
		Sequence:  sequence,
		UnixNano:  record.Time.UnixNano(),
		Direction: direction,
		Origin:    origin,
		Header: &pb.Header{
			Code:       int32(header.Code),
			ClientId:   header.ClientID,
			Parameter1: header.Parameter1,
			Parameter2: header.Parameter2,
			Length:     uint32(header.Length),
			Checksum:   uint32(header.Checksum),
		},
		Name:        decoded.Name,
		Status:      status,
		Fields:      fields,
		Consumed:    uint32(decoded.Consumed),
		Total:       uint32(decoded.Total),
		Remaining:   uint32(len(decoded.Remaining)),
		Compression: compression,
	}
	if err := contract.ValidatePacket(event); err != nil {
		return nil, err
	}

	return event, nil
}
