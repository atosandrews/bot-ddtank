package contract

import (
	"errors"
	"testing"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func TestBotBoundaryRejectsMalformedMessages(t *testing.T) {
	tests := []struct {
		name    string
		message *pb.BotMessage
	}{
		{"missing", nil},
		{"unknown variant", &pb.BotMessage{}},
		{"missing heartbeat", &pb.BotMessage{Payload: &pb.BotMessage_Heartbeat{}}},
		{"unknown failure", &pb.BotMessage{Payload: &pb.BotMessage_Failure{Failure: &pb.Failure{Code: pb.FailureCode(999)}}}},
		{"empty result", &pb.BotMessage{Payload: &pb.BotMessage_Result{Result: &pb.ScriptResult{RequestId: "id"}}}},
		{"empty batch", &pb.BotMessage{Payload: &pb.BotMessage_Batch{Batch: &pb.CommandBatch{Id: 1, Generation: 1}}}},
		{"unsupported version", &pb.BotMessage{Payload: &pb.BotMessage_Opened{Opened: &pb.SessionOpened{
			BotSessionId:  "bot",
			Generation:    1,
			ApiVersion:    999,
			CodecRevision: CodecRevision,
		}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var domain *Error
			if err := ValidateBot(tt.message); !errors.As(err, &domain) {
				t.Fatalf("error = %v, want contract error", err)
			}
		})
	}
}
