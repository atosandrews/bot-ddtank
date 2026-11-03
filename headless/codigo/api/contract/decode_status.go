package contract

import (
	"fmt"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func DecodeStatusName(value pb.DecodeStatus) (string, error) {
	switch value {
	case pb.DecodeStatus_DECODE_STATUS_DECODED:
		return "decoded", nil
	case pb.DecodeStatus_DECODE_STATUS_PARTIAL:
		return "partial", nil
	case pb.DecodeStatus_DECODE_STATUS_UNKNOWN:
		return "unknown", nil
	case pb.DecodeStatus_DECODE_STATUS_INVALID:
		return "invalid", nil
	default:
		return "", &Error{Field: "DecodeStatus", Reason: fmt.Sprintf("unsupported value %d", value)}
	}
}

func ParseDecodeStatus(value string) (pb.DecodeStatus, error) {
	switch value {
	case "decoded":
		return pb.DecodeStatus_DECODE_STATUS_DECODED, nil
	case "partial":
		return pb.DecodeStatus_DECODE_STATUS_PARTIAL, nil
	case "unknown":
		return pb.DecodeStatus_DECODE_STATUS_UNKNOWN, nil
	case "invalid":
		return pb.DecodeStatus_DECODE_STATUS_INVALID, nil
	default:
		return pb.DecodeStatus_DECODE_STATUS_UNSPECIFIED, &Error{Field: "DecodeStatus", Reason: fmt.Sprintf("unsupported value %q", value)}
	}
}
