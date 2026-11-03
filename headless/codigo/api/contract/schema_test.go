package contract

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func TestClosedContractFieldsAreEnums(t *testing.T) {
	tests := []struct {
		name    string
		message protoreflect.MessageDescriptor
		field   protoreflect.Name
	}{
		{"packet direction", (&pb.PacketEvent{}).ProtoReflect().Descriptor(), "direction"},
		{"packet origin", (&pb.PacketEvent{}).ProtoReflect().Descriptor(), "origin"},
		{"decode status", (&pb.PacketEvent{}).ProtoReflect().Descriptor(), "status"},
		{"compression", (&pb.PacketEvent{}).ProtoReflect().Descriptor(), "compression"},
		{"receipt status", (&pb.CommandReceipt{}).ProtoReflect().Descriptor(), "status"},
		{"failure code", (&pb.Failure{}).ProtoReflect().Descriptor(), "code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := tt.message.Fields().ByName(tt.field)
			if field.Kind() != protoreflect.EnumKind {
				t.Fatalf("kind = %s, want enum", field.Kind())
			}

			if field.Enum().Values().Get(0).Number() != 0 {
				t.Fatal("enum must define its unspecified zero value")
			}
		})
	}
}

func TestInitialContractVersion(t *testing.T) {
	if APIVersion != 1 {
		t.Fatalf("API version = %d, want 1", APIVersion)
	}
	if pb.Bot_GameSession_FullMethodName != "/ddbot.v1.Bot/GameSession" {
		t.Fatalf("service method = %q, want /ddbot.v1.Bot/GameSession", pb.Bot_GameSession_FullMethodName)
	}
}
