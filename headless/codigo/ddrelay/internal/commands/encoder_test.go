package commands

import (
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"testing"

	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/packets"
)

func TestCommandWireLayouts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		command Command
		code    int16
		body    string
	}{
		{"scene login", SceneLogin{SceneID: 2}, 16, "00000002"},
		{"scene state", SceneState{State: 1}, 251, "00000001"},
		{"shoot", Shoot{X: 1, Y: 2, Force: 3, Angle: -45}, 91, "02000000010000000200000003ffffffd3"},
		{"direction", Direction{Direction: -1}, 91, "07ffffffff"},
		{"move start", MoveStart{Moving: true, MoveType: 1, X: 10, Y: 20, Direction: -1, Living: true, TurnNumber: 3}, 91, "0901010000000a00000014ff010003"},
		{"move stop", MoveStop{X: 1, Y: 2, MoveFlag: true}, 91, "0a000000010000000201"},
		{"skip", SkipTurn{Argument: 0}, 91, "0c00"},
		{"pow", Pow{}, 91, "0f00"},
		{"prop", UseProp{BagType: 1, Place: 2, TemplateID: 3}, 91, "20010000000200000003"},
		{"load", LoadProgress{Progress: 100}, 91, "1000000064"},
		{"shoot start", ShootTag{Started: true, ShotType: 1}, 91, "600101"},
		{"shoot stop", ShootTag{}, 91, "6000"},
		{"mission ready", MissionReady{Ready: true}, 91, "7401"},
		{"mission start", MissionStart{Flag: true}, 82, "01"},
		{"room create", RoomCreate{RoomType: 4, TimeType: 2, NameValue: "Sala"}, 94, "000000000402000453616c610000"},
		{"room join search", RoomJoin{LoginFlag: true, RoomType: 4, RoomID: -1, Argument: 5}, 94, "000000010100000004ffffffff000000050000"},
		{"room join id", RoomJoin{RoomType: 4, RoomID: 10}, 94, "0000000100000000040000000a"},
		{"room setup", RoomSetup{MapID: 5, RoomType: 4, NameValue: "Sala", TimeType: 2, Difficulty: 1, LevelLimits: 10, Flag1: true}, 94, "0000000200000005040000000453616c6102010000000a0100"},
		{"room leave", RoomLeave{}, 94, "00000005"},
		{"room start", RoomStart{}, 94, "00000007"},
		{"room ready", RoomReady{State: 1}, 94, "0000000f01"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body, err := hex.DecodeString(tt.body)
			if err != nil {
				t.Fatal(err)
			}

			frame, err := Encode(tt.command)
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

func TestEncodingRejectsInvalidCommands(t *testing.T) {
	for _, tt := range []struct {
		name    string
		command Command
		field   string
	}{
		{"nil", nil, "command"},
		{"typed nil", (*Shoot)(nil), "command"},
		{"room type overflow", RoomCreate{RoomType: 128}, "room_type"},
		{"time type underflow", RoomCreate{TimeType: -129}, "time_type"},
		{"setup room", RoomSetup{RoomType: -129}, "room_type"},
		{"setup time", RoomSetup{TimeType: 128}, "time_type"},
		{"setup difficulty", RoomSetup{Difficulty: 128}, "difficulty"},
		{"ready state", RoomReady{State: 128}, "state"},
		{"move type", MoveStart{MoveType: 128}, "move_type"},
		{"move direction", MoveStart{Direction: -129}, "direction"},
		{"turn overflow", MoveStart{TurnNumber: 32768}, "turn_number"},
		{"turn underflow", MoveStart{TurnNumber: -32769}, "turn_number"},
		{"skip argument", SkipTurn{Argument: 128}, "argument"},
		{"bag type", UseProp{BagType: -129}, "bag_type"},
		{"shot type", ShootTag{Started: true, ShotType: 128}, "shot_type"},
		{"utf8 name", RoomCreate{NameValue: "\xff"}, "name"},
		{"utf8 password", RoomJoin{RoomID: -1, Password: "\xff"}, "password"},
		{"string limit", RoomCreate{NameValue: strings.Repeat("x", 65536)}, "name"},
		{"frame limit", RoomCreate{NameValue: strings.Repeat("x", 65506)}, "body"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := Encode(tt.command)
			var encodingError *EncodingError
			if !errors.As(err, &encodingError) {
				t.Fatalf("error = %T %v", err, err)
			}
			if encodingError.Field != tt.field || encodingError.Reason == "" || !reflect.DeepEqual(frame, framing.Frame{}) {
				t.Fatalf("frame = %#v, error = %#v", frame, encodingError)
			}
		})
	}
}

func TestEncodingWireBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name    string
		command Command
		body    string
	}{
		{"signed minima", MoveStart{MoveType: -128, Direction: -128, TurnNumber: -32768}, "090080000000000000000080008000"},
		{"signed maxima", MoveStart{MoveType: 127, Direction: 127, TurnNumber: 32767}, "09007f00000000000000007f007fff"},
		{"utf8 byte count", RoomCreate{NameValue: "á"}, "0000000000000002c3a10000"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := Encode(tt.command)
			if err != nil {
				t.Fatal(err)
			}
			if got := hex.EncodeToString(frame.Body); got != tt.body {
				t.Fatalf("body = %s, want %s", got, tt.body)
			}
		})
	}

	frame, err := Encode(RoomCreate{NameValue: strings.Repeat("x", 65505)})
	if err != nil {
		t.Fatal(err)
	}
	if frame.Header.Length != 65535 || len(frame.Body) != 65515 {
		t.Fatalf("length = %d, body = %d", frame.Header.Length, len(frame.Body))
	}
}
