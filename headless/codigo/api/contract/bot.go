package contract

import (
	"fmt"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func ValidateBot(v *pb.BotMessage) error {
	if v == nil || proto.Size(v) > MaxMessageBytes {
		return &Error{Field: "bot envelope", Reason: "missing or oversized message"}
	}

	if err := rejectUnknown(v.ProtoReflect(), 0); err != nil {
		return err
	}

	switch m := v.Payload.(type) {
	case *pb.BotMessage_Opened:
		if m.Opened == nil || m.Opened.BotSessionId == "" || len(m.Opened.BotSessionId) > 128 || m.Opened.Generation == 0 {
			return &Error{Field: "opened", Reason: "session identity and generation required"}
		}
		if m.Opened.ApiVersion != APIVersion || m.Opened.CodecRevision != CodecRevision {
			return &Error{Field: "version", Reason: "unsupported API or codec revision"}
		}
		return nil
	case *pb.BotMessage_Applied:
		if m.Applied != nil {
			return nil
		}
	case *pb.BotMessage_Batch:
		return ValidateBatch(m.Batch)
	case *pb.BotMessage_Result:
		return ValidateResult(m.Result)
	case *pb.BotMessage_Failure:
		return ValidateFailure(m.Failure)
	case *pb.BotMessage_Heartbeat:
		if m.Heartbeat != nil {
			return nil
		}
	case *pb.BotMessage_Log:
		if m.Log != nil && len(m.Log.Message) <= MaxLogBytes && utf8.ValidString(m.Log.Message) {
			return nil
		}
	default:
		return &Error{Field: "bot envelope", Reason: fmt.Sprintf("unsupported payload %T", v.Payload)}
	}

	return &Error{Field: "bot envelope", Reason: fmt.Sprintf("invalid payload %T", v.Payload)}
}
