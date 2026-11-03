package session

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/script"
)

func TestResultLimitIncludesScriptGeneration(t *testing.T) {
	owner, err := New("bot", 1, script.Options{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	defer owner.Close()
	if _, err := owner.Bind(context.Background(), 1, nil); err != nil {
		t.Fatal(err)
	}

	owner.scriptGeneration = 1

	id := strings.Repeat("r", 128)
	sample := &pb.ScriptResult{RequestId: id, Value: &pb.Value{Kind: &pb.Value_StringValue{StringValue: strings.Repeat("x", contract.MaxResultBytes)}}}
	length := contract.MaxResultBytes - (proto.Size(sample) - contract.MaxResultBytes)
	message := &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: id, Source: fmt.Sprintf(`return string.rep("x", %d)`, length)}}}
	messages, err := owner.Process(context.Background(), 1, message)
	if err != nil {
		t.Fatal(err)
	}

	result := messages[0].GetResult()
	if result.Error == nil || result.Error.Code != pb.FailureCode_FAILURE_CODE_SCRIPT_FAILED || result.Value != nil {
		t.Fatalf("oversized result: failure=%v value size=%d", result.Error, proto.Size(result.Value))
	}

	if err := contract.ValidateResult(result); err != nil {
		t.Fatal(err)
	}
}
