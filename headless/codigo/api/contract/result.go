package contract

import (
	"google.golang.org/protobuf/proto"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func ValidateResult(v *pb.ScriptResult) error {
	if v == nil || v.RequestId == "" || len(v.RequestId) > 128 || proto.Size(v) > MaxResultBytes || len(v.ActionIds) > MaxCommands {
		return &Error{Field: "result", Reason: "invalid request ID or result size"}
	}

	if v.Error != nil {
		if v.Value != nil {
			return &Error{Field: "result", Reason: "failed result must not contain a value"}
		}

		return ValidateFailure(v.Error)
	}

	if v.Value == nil {
		return &Error{Field: "result", Reason: "successful result requires a value"}
	}

	_, err := DecodeValue(v.Value)

	return err
}
