package session

import (
	"context"
	"reflect"
	"testing"
	"time"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/bot/internal/script"
)

func TestResumeReconcilesCommandOutcomes(t *testing.T) {
	for _, test := range []struct {
		name    string
		high    uint64
		receipt *pb.CommandReceipt
		want    string
		valid   bool
	}{
		{"never started", 0, nil, "cancelled", true},
		{"receipt lost", 1, nil, "uncertain", false},
		{"sent receipt retained", 1, &pb.CommandReceipt{
			BatchId:      1,
			CommandId:    1,
			Status:       pb.ReceiptStatus_RECEIPT_STATUS_SENT,
			BytesWritten: 24,
		}, "sent", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			s, err := New("session", 1, script.Options{}, time.Second)
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

			if _, err := s.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Packet{Packet: &pb.PacketEvent{
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

			if _, err := s.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "fire", Source: `return room.start()`}}}); err != nil {
				t.Fatal(err)
			}

			if err := s.Detach(ctx, 1); err != nil {
				t.Fatal(err)
			}

			resume := &pb.ResumeSession{CommandHighWater: test.high, EventCheckpoint: 1}
			if test.receipt != nil {
				resume.Receipts = []*pb.CommandReceipt{test.receipt}
			}

			if _, err := s.Bind(ctx, 2, resume); err != nil {
				t.Fatal(err)
			}

			response, err := s.Process(ctx, 2, &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "check", Source: `return {status=actions.get(1).status,valid=session.info().valid}`}}})
			if err != nil {
				t.Fatal(err)
			}

			value, err := contract.DecodeValue(response[0].GetResult().Value)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(value, map[string]any{"status": test.want, "valid": test.valid}) {
				t.Fatalf("recovery %v", value)
			}
		})
	}
}

func TestFailedEvaluationCancelsItsCommands(t *testing.T) {
	ctx := context.Background()
	s, err := New("session", 1, script.Options{}, time.Second)
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

	if _, err := s.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Packet{Packet: &pb.PacketEvent{
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

	for i, source := range []string{`room.start();error("failure")`, `room.start();return function() end`} {
		id := []string{"throw", "serialize"}[i]
		response, err := s.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: id, Source: source}}})
		if err != nil {
			t.Fatal(err)
		}

		if len(response) != 1 || response[0].GetResult().Error == nil || !reflect.DeepEqual(response[0].GetResult().ActionIds, []uint64{uint64(i + 1)}) {
			t.Fatalf("failed evaluation %v", response)
		}
	}

	response, err := s.Process(ctx, 1, &pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "check", Source: `return {actions.get(1).status,actions.get(2).status}`}}})
	if err != nil {
		t.Fatal(err)
	}

	value, err := contract.DecodeValue(response[0].GetResult().Value)
	if err != nil || !reflect.DeepEqual(value, []any{"cancelled", "cancelled"}) {
		t.Fatalf("actions %v %v", value, err)
	}
}
