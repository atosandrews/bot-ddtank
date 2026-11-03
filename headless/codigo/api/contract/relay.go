package contract

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"google.golang.org/protobuf/proto"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func ValidateRelay(v *pb.RelayMessage) error {
	if v == nil || proto.Size(v) > MaxMessageBytes || len(v.ProtoReflect().GetUnknown()) != 0 {
		return &Error{Field: "envelope", Reason: "missing, unknown or oversized message"}
	}

	if err := rejectUnknown(v.ProtoReflect(), 0); err != nil {
		return err
	}

	switch m := v.Payload.(type) {
	case *pb.RelayMessage_Open:
		return ValidateOpen(m.Open)
	case *pb.RelayMessage_Resume:
		if m.Resume == nil || m.Resume.BotSessionId == "" || m.Resume.Generation == 0 || len(m.Resume.Receipts) > 128 {
			return &Error{Field: "resume", Reason: "invalid resume identity or receipts"}
		}
		if err := ValidateOpen(m.Resume.Identity); err != nil {
			return err
		}
		for _, receipt := range m.Resume.Receipts {
			if err := ValidateReceipt(receipt); err != nil {
				return err
			}
		}
		return nil
	case *pb.RelayMessage_Packet:
		return ValidatePacket(m.Packet)
	case *pb.RelayMessage_Evaluate:
		if m.Evaluate == nil {
			return &Error{Field: "evaluate", Reason: "payload required"}
		}
		return validateSource(m.Evaluate.RequestId, m.Evaluate.Source, MaxEvalBytes)
	case *pb.RelayMessage_Load:
		if m.Load == nil {
			return &Error{Field: "load", Reason: "payload required"}
		}
		if err := validateSource(m.Load.RequestId, m.Load.Source, MaxScriptBytes); err != nil {
			return err
		}
		hash := sha256.Sum256([]byte(m.Load.Source))
		if !bytes.Equal(hash[:], m.Load.Sha256) {
			return &Error{Field: "sha256", Reason: "source hash mismatch"}
		}
		return nil
	case *pb.RelayMessage_Stop:
		if m.Stop == nil || len(m.Stop.RequestId) == 0 || len(m.Stop.RequestId) > 128 {
			return &Error{Field: "stop", Reason: "request ID required"}
		}
		return nil
	case *pb.RelayMessage_Receipt:
		return ValidateReceipt(m.Receipt)
	case *pb.RelayMessage_Gap:
		if m.Gap != nil && len(m.Gap.Reason) <= 4096 {
			return nil
		}
	case *pb.RelayMessage_Close:
		if m.Close != nil && len(m.Close.Reason) <= 4096 {
			return nil
		}
	case *pb.RelayMessage_Heartbeat:
		if m.Heartbeat != nil {
			return nil
		}
	default:
		return &Error{Field: "envelope", Reason: fmt.Sprintf("unsupported payload %T", v.Payload)}
	}

	return &Error{Field: "envelope", Reason: "invalid payload"}
}
