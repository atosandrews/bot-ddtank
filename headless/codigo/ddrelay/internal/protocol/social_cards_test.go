package protocol

import (
	"bytes"
	"testing"

	pb "ddbot/api/gen/go/ddbot/v1"
)

func TestSocialAndCardWireCommands(t *testing.T) {
	tests := []struct {
		name    string
		command *pb.Command
		code    int16
		body    []byte
	}{
		{"friend", &pb.Command{Payload: &pb.Command_FriendAdd{FriendAdd: &pb.FriendAdd{
			Name:     "Pal",
			Relation: 0,
			Flag1:    false,
			Flag2:    true,
		}}}, 160, []byte{160, 0, 3, 'P', 'a', 'l', 0, 0, 0, 0, 0, 1}},
		{"invite", &pb.Command{Payload: &pb.Command_RoomInvite{RoomInvite: &pb.RoomInvite{PlayerId: 42}}}, 70, []byte{0, 0, 0, 42}},
		{"normal", &pb.Command{Payload: &pb.Command_CardTake{CardTake: &pb.CardTake{Index: 20}}}, 91, []byte{98, 20}},
		{"boss", &pb.Command{Payload: &pb.Command_CardTakeBoss{CardTakeBoss: &pb.CardTakeBoss{Index: 8}}}, 91, []byte{130, 8}},
		{"extra", &pb.Command{Payload: &pb.Command_CardTakeExtra{CardTakeExtra: &pb.CardTakeExtra{Index: 3}}}, 91, []byte{114, 3}},
		{"finish", &pb.Command{Payload: &pb.Command_CardFinish{CardFinish: &pb.CardFinish{}}}, 91, []byte{98, 100}},
		{"finish boss", &pb.Command{Payload: &pb.Command_CardFinishBoss{CardFinishBoss: &pb.CardFinishBoss{}}}, 91, []byte{130, 100}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := Encode(tt.command)
			if err != nil {
				t.Fatal(err)
			}

			if frame.Header.Code != tt.code || int(frame.Header.Length) != 20+len(tt.body) || !bytes.Equal(frame.Body, tt.body) {
				t.Fatalf("frame=%+v want code=%d body=%x", frame, tt.code, tt.body)
			}
		})
	}
}
