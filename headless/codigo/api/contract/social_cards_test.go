package contract

import (
	"strings"
	"testing"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func TestSocialAndCardCommands(t *testing.T) {
	tests := []struct {
		name    string
		command *pb.Command
		valid   bool
	}{
		{"pow", &pb.Command{Payload: &pb.Command_Pow{Pow: &pb.Pow{}}}, true},
		{"missing pow", &pb.Command{Payload: &pb.Command_Pow{}}, false},
		{"friend", &pb.Command{Payload: &pb.Command_FriendAdd{FriendAdd: &pb.FriendAdd{Name: "Friend"}}}, true},
		{"empty friend", &pb.Command{Payload: &pb.Command_FriendAdd{FriendAdd: &pb.FriendAdd{}}}, false},
		{"large friend", &pb.Command{Payload: &pb.Command_FriendAdd{FriendAdd: &pb.FriendAdd{Name: strings.Repeat("a", 65535)}}}, false},
		{"missing friend", &pb.Command{Payload: &pb.Command_FriendAdd{}}, false},
		{"invite", &pb.Command{Payload: &pb.Command_RoomInvite{RoomInvite: &pb.RoomInvite{PlayerId: 42}}}, true},
		{"invalid invite", &pb.Command{Payload: &pb.Command_RoomInvite{RoomInvite: &pb.RoomInvite{PlayerId: 0}}}, false},
		{"card", &pb.Command{Payload: &pb.Command_CardTake{CardTake: &pb.CardTake{Index: 20}}}, true},
		{"card sentinel", &pb.Command{Payload: &pb.Command_CardTake{CardTake: &pb.CardTake{Index: 100}}}, false},
		{"negative card", &pb.Command{Payload: &pb.Command_CardTake{CardTake: &pb.CardTake{Index: -1}}}, false},
		{"boss", &pb.Command{Payload: &pb.Command_CardTakeBoss{CardTakeBoss: &pb.CardTakeBoss{Index: 8}}}, true},
		{"boss overflow", &pb.Command{Payload: &pb.Command_CardTakeBoss{CardTakeBoss: &pb.CardTakeBoss{Index: 9}}}, false},
		{"extra", &pb.Command{Payload: &pb.Command_CardTakeExtra{CardTakeExtra: &pb.CardTakeExtra{Index: 0}}}, true},
		{"extra overflow", &pb.Command{Payload: &pb.Command_CardTakeExtra{CardTakeExtra: &pb.CardTakeExtra{Index: 21}}}, false},
		{"finish", &pb.Command{Payload: &pb.Command_CardFinish{CardFinish: &pb.CardFinish{}}}, true},
		{"finish boss", &pb.Command{Payload: &pb.Command_CardFinishBoss{CardFinishBoss: &pb.CardFinishBoss{}}}, true},
		{"missing finish", &pb.Command{Payload: &pb.Command_CardFinish{}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommand(tt.command)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
		})
	}
}
