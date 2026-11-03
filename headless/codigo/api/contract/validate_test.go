package contract

import (
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func TestPacketRejectsUnspecifiedAndUnknownEnums(t *testing.T) {
	for _, field := range []protoreflect.Name{"direction", "origin", "status", "compression"} {
		for _, value := range []protoreflect.EnumNumber{0, 999} {
			t.Run(string(field)+"/"+valueString(value), func(t *testing.T) {
				packet := &pb.PacketEvent{
					Sequence:    1,
					Direction:   pb.PacketDirection_PACKET_DIRECTION_SERVER_TO_CLIENT,
					Origin:      pb.PacketOrigin_PACKET_ORIGIN_NETWORK,
					Status:      pb.DecodeStatus_DECODE_STATUS_DECODED,
					Compression: pb.Compression_COMPRESSION_NONE,
					Header:      &pb.Header{},
					Fields:      &pb.Object{},
				}
				reflected := packet.ProtoReflect()
				reflected.Set(reflected.Descriptor().Fields().ByName(field), protoreflect.ValueOfEnum(value))
				var domain *Error
				if err := ValidatePacket(packet); !errors.As(err, &domain) {
					t.Fatalf("error = %v, want contract error", err)
				}
			})
		}
	}
}

func TestResumeValidatesEveryReceiptBeforeBinding(t *testing.T) {
	identity := &pb.OpenSession{
		ApiVersion:      APIVersion,
		CodecRevision:   CodecRevision,
		RelayInstanceId: "relay",
		GameSessionId:   "game",
		LocalStream:     1,
	}

	for _, receipt := range []*pb.CommandReceipt{
		nil,
		{BatchId: 1, CommandId: 1},
		{BatchId: 1, CommandId: 1, Status: pb.ReceiptStatus(999)},
		{
			BatchId:   1,
			CommandId: 1,
			Status:    pb.ReceiptStatus_RECEIPT_STATUS_REJECTED,
			Error:     &pb.Failure{},
		},
	} {
		message := &pb.RelayMessage{Payload: &pb.RelayMessage_Resume{Resume: &pb.ResumeSession{
			Identity: identity, BotSessionId: "bot", Generation: 1, Receipts: []*pb.CommandReceipt{receipt},
		}}}
		var domain *Error
		if err := ValidateRelay(message); !errors.As(err, &domain) {
			t.Fatalf("receipt %v: error = %v, want contract error", receipt, err)
		}
	}
}

func TestUnsupportedVersionIsRejected(t *testing.T) {
	opening := &pb.OpenSession{
		ApiVersion:      999,
		CodecRevision:   CodecRevision,
		RelayInstanceId: "relay",
		GameSessionId:   "game",
		LocalStream:     1,
	}
	var domain *Error
	if err := ValidateOpen(opening); !errors.As(err, &domain) || domain.Field != "version" {
		t.Fatalf("error = %v, want version error", err)
	}
}

func TestEnumPacketRoundTrip(t *testing.T) {
	want := &pb.PacketEvent{Sequence: 7, Direction: pb.PacketDirection_PACKET_DIRECTION_CLIENT_TO_SERVER,
		Origin: pb.PacketOrigin_PACKET_ORIGIN_SCRIPT, Status: pb.DecodeStatus_DECODE_STATUS_PARTIAL,
		Compression: pb.Compression_COMPRESSION_ZLIB, Header: &pb.Header{Code: 91}, Fields: &pb.Object{}, Total: 12, Consumed: 8, Remaining: 4, CommandId: 9}
	data, err := proto.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}

	got := &pb.PacketEvent{}
	if err := proto.Unmarshal(data, got); err != nil {
		t.Fatal(err)
	}

	if err := ValidatePacket(got); err != nil {
		t.Fatal(err)
	}

	if !proto.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func valueString(value protoreflect.EnumNumber) string {
	if value == 0 {
		return "unspecified"
	}

	return "unknown"
}
