package contract

import (
	"math"

	"google.golang.org/protobuf/proto"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func ValidatePacket(v *pb.PacketEvent) error {
	if v == nil || v.Sequence == 0 || v.Header == nil || v.Fields == nil {
		return &Error{Field: "packet", Reason: "sequence, header and fields required"}
	}

	if proto.Size(v) > MaxMessageBytes {
		return &Error{Field: "packet", Reason: "message too large"}
	}

	if _, err := PacketDirectionName(v.Direction); err != nil {
		return err
	}

	if _, err := PacketOriginName(v.Origin); err != nil {
		return err
	}

	if _, err := DecodeStatusName(v.Status); err != nil {
		return err
	}

	if _, err := CompressionName(v.Compression); err != nil {
		return err
	}

	if v.Header.Code < math.MinInt16 || v.Header.Code > math.MaxInt16 || v.Header.Length > math.MaxUint16 || v.Header.Checksum > math.MaxUint16 {
		return &Error{Field: "header", Reason: "integer exceeds wire range"}
	}

	if v.Consumed > v.Total || v.Remaining != v.Total-v.Consumed {
		return &Error{Field: "consumption", Reason: "inconsistent byte counts"}
	}

	_, err := DecodeFields(v.Fields)

	return err
}
