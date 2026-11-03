package session

import (
	"context"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/script"
)

func TestSessionEvaluationAndReplacement(t *testing.T) {
	ctx := context.Background()
	s, err := New("bot-one", 1, script.Options{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	defer s.Close()
	if _, err := s.Bind(ctx, 1, nil); err != nil {
		t.Fatal(err)
	}

	fields, err := contract.EncodeFields(map[string]any{"status": int32(0)})
	if err != nil {
		t.Fatal(err)
	}

	login := &pb.RelayMessage{Payload: &pb.RelayMessage_Packet{Packet: &pb.PacketEvent{
		Sequence:    1,
		Direction:   pb.PacketDirection_PACKET_DIRECTION_SERVER_TO_CLIENT,
		Origin:      pb.PacketOrigin_PACKET_ORIGIN_NETWORK,
		Compression: pb.Compression_COMPRESSION_NONE,
		Header:      &pb.Header{Code: 1, ClientId: 123},
		Status:      pb.DecodeStatus_DECODE_STATUS_DECODED,
		Fields:      fields,
	}}}
	if _, err := s.Process(ctx, 1, login); err != nil {
		t.Fatal(err)
	}

	source := `return {api_version=1,on_packet=function(p) count=(count or 0)+1 end}`
	hash := sha256.Sum256([]byte(source))
	load := &pb.RelayMessage{Payload: &pb.RelayMessage_Load{Load: &pb.LoadScript{RequestId: "load", Source: source, Sha256: hash[:]}}}
	result, err := s.Process(ctx, 1, load)
	if err != nil || result[0].GetResult().Error != nil {
		t.Fatalf("load %v %v", result, err)
	}

	eval := &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "eval", Source: `return room.start()`}}}
	result, err = s.Process(ctx, 1, eval)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 2 || result[0].GetBatch().GetCommands()[0].GetRoomStart() == nil || result[1].GetResult().GetActionIds()[0] != 1 {
		t.Fatalf("result %v", result)
	}

	again, err := s.Process(ctx, 1, eval)
	if err != nil || len(again) != 1 || again[0].GetResult().GetActionIds()[0] != 1 {
		t.Fatalf("duplicate %v %v", again, err)
	}

	eval.GetEvaluate().Source = `return 2`
	conflict, err := s.Process(ctx, 1, eval)
	if err != nil {
		t.Fatal(err)
	}

	failure := conflict[0].GetResult().GetError()
	if len(conflict) != 1 || failure == nil || failure.GetCode() != pb.FailureCode_FAILURE_CODE_SCRIPT_FAILED || !strings.Contains(failure.GetMessage(), "request_conflict") {
		t.Fatalf("changed source %v", conflict)
	}

	bad := `room.start(); return {api_version=1}`
	hash = sha256.Sum256([]byte(bad))
	load.GetLoad().RequestId = "bad"
	load.GetLoad().Source = bad
	load.GetLoad().Sha256 = hash[:]
	result, err = s.Process(ctx, 1, load)
	if err != nil || result[0].GetResult().Error == nil {
		t.Fatalf("accepted initial command %v %v", result, err)
	}

	result, err = s.Process(ctx, 1, login)
	if err != nil || result[0].GetApplied().Sequence != 1 {
		t.Fatalf("duplicate packet %v %v", result, err)
	}

	login.GetPacket().Sequence = 3
	if _, err = s.Process(ctx, 1, login); err == nil {
		t.Fatal("accepted sequence gap")
	}
}
