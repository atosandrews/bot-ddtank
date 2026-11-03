package protocol

import (
	"encoding/hex"
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/packets"
)

func TestCommandWireLayouts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		command *pb.Command
		code    int16
		body    string
	}{
		{"scene login", &pb.Command{Id: 1, Payload: &pb.Command_SceneLogin{SceneLogin: &pb.SceneLogin{SceneId: 2}}}, 16, "00000002"},
		{"scene state", &pb.Command{Id: 1, Payload: &pb.Command_SceneState{SceneState: &pb.SceneState{State: 1}}}, 251, "00000001"},
		{"shoot", &pb.Command{Id: 1, Payload: &pb.Command_Shoot{Shoot: &pb.Shoot{
			X:     1,
			Y:     2,
			Force: 3,
			Angle: -45,
		}}}, 91, "02000000010000000200000003ffffffd3"},
		{"direction", &pb.Command{Id: 1, Payload: &pb.Command_Direction{Direction: &pb.Direction{Direction: -1}}}, 91, "07ffffffff"},
		{"move start", &pb.Command{Id: 1, Payload: &pb.Command_MoveStart{MoveStart: &pb.MoveStart{
			Moving:     true,
			MoveType:   1,
			X:          10,
			Y:          20,
			Direction:  -1,
			Living:     true,
			TurnNumber: 3,
		}}}, 91, "0901010000000a00000014ff010003"},
		{"move stop", &pb.Command{Id: 1, Payload: &pb.Command_MoveStop{MoveStop: &pb.MoveStop{X: 1, Y: 2, MoveFlag: true}}}, 91, "0a000000010000000201"},
		{"skip", &pb.Command{Id: 1, Payload: &pb.Command_SkipTurn{SkipTurn: &pb.SkipTurn{Argument: 0}}}, 91, "0c00"},
		{"pow", &pb.Command{Id: 1, Payload: &pb.Command_Pow{Pow: &pb.Pow{}}}, 91, "0f00"},
		{"prop", &pb.Command{Id: 1, Payload: &pb.Command_UseProp{UseProp: &pb.UseProp{BagType: 1, Place: 2, TemplateId: 3}}}, 91, "20010000000200000003"},
		{"load", &pb.Command{Id: 1, Payload: &pb.Command_LoadProgress{LoadProgress: &pb.LoadProgress{Progress: 100}}}, 91, "1000000064"},
		{"shoot start", &pb.Command{Id: 1, Payload: &pb.Command_ShootTag{ShootTag: &pb.ShootTag{Started: true, ShotType: 1}}}, 91, "600101"},
		{"shoot stop", &pb.Command{Id: 1, Payload: &pb.Command_ShootTag{ShootTag: &pb.ShootTag{}}}, 91, "6000"},
		{"mission ready", &pb.Command{Id: 1, Payload: &pb.Command_MissionReady{MissionReady: &pb.MissionReady{Ready: true}}}, 91, "7401"},
		{"mission start", &pb.Command{Id: 1, Payload: &pb.Command_MissionStart{MissionStart: &pb.MissionStart{Flag: true}}}, 82, "01"},
		{"room create", &pb.Command{Id: 1, Payload: &pb.Command_RoomCreate{RoomCreate: &pb.RoomCreate{RoomType: 4, TimeType: 2, NameValue: "Sala"}}}, 94, "000000000402000453616c610000"},
		{"room join search", &pb.Command{Id: 1, Payload: &pb.Command_RoomJoin{RoomJoin: &pb.RoomJoin{
			LoginFlag: true,
			RoomType:  4,
			RoomId:    -1,
			Argument:  5,
		}}}, 94, "000000010100000004ffffffff000000050000"},
		{"room join id", &pb.Command{Id: 1, Payload: &pb.Command_RoomJoin{RoomJoin: &pb.RoomJoin{RoomType: 4, RoomId: 10}}}, 94, "0000000100000000040000000a"},
		{"room setup", &pb.Command{Id: 1, Payload: &pb.Command_RoomSetup{RoomSetup: &pb.RoomSetup{
			MapId:       5,
			RoomType:    4,
			NameValue:   "Sala",
			TimeType:    2,
			Difficulty:  1,
			LevelLimits: 10,
			Flag1:       true,
		}}}, 94, "0000000200000005040000000453616c6102010000000a0100"},
		{"room leave", &pb.Command{Id: 1, Payload: &pb.Command_RoomLeave{RoomLeave: &pb.RoomLeave{}}}, 94, "00000005"},
		{"room start", &pb.Command{Id: 1, Payload: &pb.Command_RoomStart{RoomStart: &pb.RoomStart{}}}, 94, "00000007"},
		{"room ready", &pb.Command{Id: 1, Payload: &pb.Command_RoomReady{RoomReady: &pb.RoomReady{State: 1}}}, 94, "0000000f01"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body, err := hex.DecodeString(tt.body)
			if err != nil {
				t.Fatal(err)
			}

			wire, err := proto.Marshal(tt.command)
			if err != nil {
				t.Fatal(err)
			}

			received := new(pb.Command)
			if err := proto.Unmarshal(wire, received); err != nil {
				t.Fatal(err)
			}

			frame, err := Encode(received)
			if err != nil {
				t.Fatal(err)
			}

			want := framing.Frame{Header: framing.Header{Code: tt.code, Length: uint16(framing.HeaderSize + len(body))}, Body: body}
			if !reflect.DeepEqual(frame, want) {
				t.Fatalf("frame = %#v, want %#v", frame, want)
			}

			decoded := packets.NewDecoder().Decode(capture.ClientToServer, frame)
			if decoded.Status != "decoded" || decoded.Consumed != len(body) || len(decoded.Remaining) != 0 {
				t.Fatalf("decoded = %#v", decoded)
			}
		})
	}
}
