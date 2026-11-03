package contract

import (
	pb "ddbot/api/gen/go/ddbot/v1"
)

func ValidateReceipt(v *pb.CommandReceipt) error {
	if v == nil || v.CommandId == 0 || v.BatchId == 0 {
		return &Error{Field: "receipt", Reason: "batch and command IDs required"}
	}

	if _, err := ReceiptStatusName(v.Status); err != nil {
		return err
	}

	if v.Error != nil {
		return ValidateFailure(v.Error)
	}

	return nil
}
