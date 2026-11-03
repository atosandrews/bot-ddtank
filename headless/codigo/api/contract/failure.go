package contract

import (
	"fmt"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func ValidateFailure(v *pb.Failure) error {
	if v == nil || len(v.Message) > 4096 {
		return &Error{Field: "failure", Reason: "failure required with message at most 4096 bytes"}
	}

	_, err := FailureCodeName(v.Code)

	return err
}

func FailureCodeName(value pb.FailureCode) (string, error) {
	switch value {
	case pb.FailureCode_FAILURE_CODE_SCRIPT_FAILED:
		return "script_failed", nil
	case pb.FailureCode_FAILURE_CODE_STALE_BINDING:
		return "stale_binding", nil
	case pb.FailureCode_FAILURE_CODE_STALE_STATE:
		return "stale_state", nil
	case pb.FailureCode_FAILURE_CODE_PACKET_BOUNDARY_UNAVAILABLE:
		return "packet_boundary_unavailable", nil
	case pb.FailureCode_FAILURE_CODE_COMMAND_ALREADY_SEEN:
		return "command_already_seen", nil
	case pb.FailureCode_FAILURE_CODE_INVALID_COMMAND:
		return "invalid_command", nil
	case pb.FailureCode_FAILURE_CODE_SCRIPT_OR_BINDING_CHANGED:
		return "script_or_binding_changed", nil
	case pb.FailureCode_FAILURE_CODE_WRITE_FAILED:
		return "write_failed", nil
	case pb.FailureCode_FAILURE_CODE_OUTCOME_EXPIRED:
		return "outcome_expired", nil
	default:
		return "", &Error{Field: "failure code", Reason: fmt.Sprintf("unsupported value %d", value)}
	}
}
