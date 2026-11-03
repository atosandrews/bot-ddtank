package session

import (
	"context"
	"testing"
	"time"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/script"
)

func TestSlowSessionDoesNotBlockRegistryOrAnotherOwner(t *testing.T) {
	registry, err := NewRegistry(script.Options{}, Limits{
		Sessions:  4,
		PerTenant: 4,
		PerRelay:  4,
		Grace:     time.Second,
		Timeout:   250 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer registry.Close()
	identity := func(id string) *pb.OpenSession {
		return &pb.OpenSession{
			ApiVersion:      contract.APIVersion,
			CodecRevision:   contract.CodecRevision,
			RelayInstanceId: "relay",
			GameSessionId:   id,
			LocalStream:     1,
		}
	}
	slow, _, _, err := registry.Open(context.Background(), "tenant", identity("slow"), nil)
	if err != nil {
		t.Fatal(err)
	}

	finished := make(chan reply, 1)
	go func() {
		result, err := slow.Process(context.Background(), 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "slow", Source: `while true do end`}}})
		finished <- reply{messages: result, err: err}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	healthy, _, _, err := registry.Open(ctx, "tenant", identity("healthy"), nil)
	if err != nil {
		t.Fatal(err)
	}

	result, err := healthy.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "healthy", Source: `return 42`}}})
	if err != nil || len(result) != 1 || result[0].GetResult().GetError() != nil {
		t.Fatalf("healthy result=%v error=%v", result, err)
	}

	value, err := contract.DecodeValue(result[0].GetResult().Value)
	if err != nil || value != float64(42) {
		t.Fatalf("value=%v error=%v", value, err)
	}

	select {
	case response := <-finished:
		if response.err != nil {
			t.Fatal(response.err)
		}
		result := response.messages
		if len(result) != 1 || result[0].GetResult().GetError() == nil {
			t.Fatalf("slow script missed deadline: %v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("slow script did not stop")
	}
}
