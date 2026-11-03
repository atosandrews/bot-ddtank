package session

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/script"
)

func TestFailedReplacementRetainsGlobalsAndStopResetsThem(t *testing.T) {
	s, err := New("session", 1, script.Options{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	defer s.Close()
	ctx := context.Background()
	if _, err := s.Bind(ctx, 1, nil); err != nil {
		t.Fatal(err)
	}

	load := func(id, source string) *pb.ScriptResult {
		t.Helper()
		hash := sha256.Sum256([]byte(source))
		messages, err := s.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Load{Load: &pb.LoadScript{RequestId: id, Source: source, Sha256: hash[:]}}})
		if err != nil || len(messages) != 1 {
			t.Fatalf("load=%v error=%v", messages, err)
		}

		return messages[0].GetResult()
	}
	if result := load("good", `marker=42;return {api_version=1}`); result.Error != nil || result.ScriptGeneration != 1 {
		t.Fatalf("good=%v", result)
	}

	if result := load("bad", `marker=99;return {invalid=true}`); result.Error == nil || result.ScriptGeneration != 1 {
		t.Fatalf("bad=%v", result)
	}

	eval := func(id, source string) any {
		t.Helper()
		messages, err := s.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: id, Source: source}}})
		if err != nil || len(messages) != 1 || messages[0].GetResult().Error != nil {
			t.Fatalf("eval=%v error=%v", messages, err)
		}

		value, err := contract.DecodeValue(messages[0].GetResult().Value)
		if err != nil {
			t.Fatal(err)
		}

		return value
	}
	if value := eval("before", "return marker"); value != float64(42) {
		t.Fatalf("old globals=%v", value)
	}

	messages, err := s.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Stop{Stop: &pb.StopScript{RequestId: "stop"}}})
	if err != nil || messages[0].GetResult().ScriptGeneration != 2 {
		t.Fatalf("stop=%v error=%v", messages, err)
	}

	if value := eval("after", "return marker==nil"); value != true {
		t.Fatalf("stop retained globals=%v", value)
	}
}
