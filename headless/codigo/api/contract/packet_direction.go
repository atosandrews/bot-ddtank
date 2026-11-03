package contract

import (
	"fmt"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func PacketDirectionName(value pb.PacketDirection) (string, error) {
	switch value {
	case pb.PacketDirection_PACKET_DIRECTION_CLIENT_TO_SERVER:
		return "ClientToServer", nil
	case pb.PacketDirection_PACKET_DIRECTION_SERVER_TO_CLIENT:
		return "ServerToClient", nil
	default:
		return "", &Error{Field: "PacketDirection", Reason: fmt.Sprintf("unsupported value %d", value)}
	}
}

func ParsePacketDirection(value string) (pb.PacketDirection, error) {
	switch value {
	case "ClientToServer":
		return pb.PacketDirection_PACKET_DIRECTION_CLIENT_TO_SERVER, nil
	case "ServerToClient":
		return pb.PacketDirection_PACKET_DIRECTION_SERVER_TO_CLIENT, nil
	default:
		return pb.PacketDirection_PACKET_DIRECTION_UNSPECIFIED, &Error{Field: "PacketDirection", Reason: fmt.Sprintf("unsupported value %q", value)}
	}
}
