package session

import (
	"context"
	"strings"
	"testing"
	"time"

	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/script"
)

func TestLuaLogsAreBoundedAndReturned(t *testing.T) {
	ctx := context.Background()
	s, err := New("logs", 1, script.Options{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	defer s.Close()
	if _, err := s.Bind(ctx, 1, nil); err != nil {
		t.Fatal(err)
	}

	messages, err := s.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "log", Source: `console.log("hello");return 1`}}})
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 2 || messages[0].GetResult().Error != nil || messages[1].GetLog().Message != "hello\n" {
		t.Fatalf("logs %v", messages)
	}

	var buffer logBuffer
	if n, err := buffer.Write([]byte(strings.Repeat("x", 8193))); n != 0 || err == nil {
		t.Fatalf("log bound %d %v", n, err)
	}
}
