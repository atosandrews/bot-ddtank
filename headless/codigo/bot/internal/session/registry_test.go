package session

import (
	"context"
	"testing"
	"time"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/script"
)

func TestTenantRelayAndGameIsolation(t *testing.T) {
	r, err := NewRegistry(script.Options{}, Limits{
		Sessions:  8,
		PerTenant: 4,
		PerRelay:  2,
		Grace:     time.Second,
		Timeout:   time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer r.Close()
	ctx := context.Background()
	identities := []struct{ tenant, relay, game string }{{"a", "one", "one"}, {"a", "one", "two"}, {"a", "two", "one"}, {"b", "one", "one"}}
	owners := make([]*Session, len(identities))
	opens := make([]*pb.OpenSession, len(identities))
	ids := make(map[string]bool)

	for i, key := range identities {
		open := &pb.OpenSession{
			ApiVersion:      contract.APIVersion,
			CodecRevision:   contract.CodecRevision,
			RelayInstanceId: key.relay,
			GameSessionId:   key.game,
			LocalStream:     1,
		}
		opens[i] = open
		owner, messages, generation, err := r.Open(ctx, key.tenant, open, nil)
		if err != nil {
			t.Fatal(err)
		}

		owners[i] = owner
		id := messages[0].GetOpened().BotSessionId
		if ids[id] || generation != 1 {
			t.Fatalf("identity collision %s %d", id, generation)
		}

		ids[id] = true
		_, err = owner.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "set", Source: `counter=41; return counter`}}})
		if err != nil {
			t.Fatal(err)
		}
	}

	for i, owner := range owners {
		fields, err := contract.EncodeFields(map[string]any{"status": int32(0)})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := owner.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Packet{Packet: &pb.PacketEvent{
			Sequence:    1,
			Direction:   pb.PacketDirection_PACKET_DIRECTION_SERVER_TO_CLIENT,
			Origin:      pb.PacketOrigin_PACKET_ORIGIN_NETWORK,
			Compression: pb.Compression_COMPRESSION_NONE,
			Header:      &pb.Header{Code: 1, ClientId: 123},
			Status:      pb.DecodeStatus_DECODE_STATUS_DECODED,
			Fields:      fields,
		}}}); err != nil {
			t.Fatal(err)
		}

		messages, err := owner.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "action", Source: `return room.start()`}}})
		if err != nil || len(messages) != 2 || messages[0].GetBatch().Commands[0].Id != 1 {
			t.Fatalf("owner %d actions=%v error=%v", i, messages, err)
		}
	}

	if _, _, _, err := r.Open(ctx, "a", opens[0], nil); err == nil {
		t.Fatal("duplicate opening replaced owner")
	}

	r.Release("a", opens[0], 1)
	resumed, messages, generation, err := r.Open(ctx, "a", opens[0], &pb.ResumeSession{Identity: opens[0], BotSessionId: owners[0].id, Generation: 1})
	if err != nil {
		t.Fatal(err)
	}

	if resumed != owners[0] || generation != 2 || messages[0].GetOpened().Generation != 2 {
		t.Fatalf("resume %p %d %v", resumed, generation, messages)
	}

	request := &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "get", Source: `counter=counter+1;return counter`}}}
	if _, err := resumed.Process(ctx, 1, request); err == nil {
		t.Fatal("old generation accepted")
	}

	for i, owner := range owners {
		generation := uint64(1)
		if i == 0 {
			generation = 2
		}

		result, err := owner.Process(ctx, generation, request)
		if err != nil {
			t.Fatal(err)
		}

		value, err := contract.DecodeValue(result[0].GetResult().Value)
		if err != nil || value != float64(42) {
			t.Fatalf("session %d value %v error %v", i, value, err)
		}
	}

	r.Release("a", opens[0], 2)
	r.Expire(time.Now().Add(2 * time.Second))
	if _, _, _, err := r.Open(ctx, "a", opens[0], &pb.ResumeSession{Identity: opens[0], BotSessionId: owners[0].id, Generation: 2}); err == nil {
		t.Fatal("expired session resumed")
	}
}

func TestStaleReleaseCannotCloseResumedSession(t *testing.T) {
	registry, err := NewRegistry(script.Options{}, Limits{
		Sessions:  2,
		PerTenant: 2,
		PerRelay:  2,
		Grace:     time.Minute,
		Timeout:   time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer registry.Close()

	identity := &pb.OpenSession{
		ApiVersion:      contract.APIVersion,
		CodecRevision:   contract.CodecRevision,
		RelayInstanceId: "relay",
		GameSessionId:   "game",
		LocalStream:     1,
	}
	owner, _, generation, err := registry.Open(context.Background(), "tenant", identity, nil)
	if err != nil {
		t.Fatal(err)
	}

	registry.Release("tenant", identity, generation)
	_, _, resumed, err := registry.Open(context.Background(), "tenant", identity, &pb.ResumeSession{Identity: identity, BotSessionId: owner.id, Generation: generation})
	if err != nil {
		t.Fatal(err)
	}

	registry.Release("tenant", identity, generation)
	messages, err := owner.Process(context.Background(), resumed, &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "still-active", Source: "return 42"}}})
	if err != nil {
		t.Fatal(err)
	}

	result := messages[0].GetResult()
	value, err := contract.DecodeValue(result.Value)
	if err != nil {
		t.Fatal(err)
	}

	if value != float64(42) {
		t.Fatalf("value = %#v, want 42", value)
	}
}
