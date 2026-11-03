package contract

import (
	"fmt"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func PacketOriginName(value pb.PacketOrigin) (string, error) {
	switch value {
	case pb.PacketOrigin_PACKET_ORIGIN_NETWORK:
		return "", nil
	case pb.PacketOrigin_PACKET_ORIGIN_SCRIPT:
		return "script", nil
	default:
		return "", &Error{Field: "PacketOrigin", Reason: fmt.Sprintf("unsupported value %d", value)}
	}
}

func ParsePacketOrigin(value string) (pb.PacketOrigin, error) {
	switch value {
	case "":
		return pb.PacketOrigin_PACKET_ORIGIN_NETWORK, nil
	case "script":
		return pb.PacketOrigin_PACKET_ORIGIN_SCRIPT, nil
	default:
		return pb.PacketOrigin_PACKET_ORIGIN_UNSPECIFIED, &Error{Field: "PacketOrigin", Reason: fmt.Sprintf("unsupported value %q", value)}
	}
}
