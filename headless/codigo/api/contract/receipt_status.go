package contract

import (
	"fmt"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func ReceiptStatusName(value pb.ReceiptStatus) (string, error) {
	switch value {
	case pb.ReceiptStatus_RECEIPT_STATUS_SENT:
		return "sent", nil
	case pb.ReceiptStatus_RECEIPT_STATUS_REJECTED:
		return "rejected", nil
	case pb.ReceiptStatus_RECEIPT_STATUS_CANCELLED:
		return "cancelled", nil
	case pb.ReceiptStatus_RECEIPT_STATUS_UNCERTAIN:
		return "uncertain", nil
	default:
		return "", &Error{Field: "ReceiptStatus", Reason: fmt.Sprintf("unsupported value %d", value)}
	}
}
