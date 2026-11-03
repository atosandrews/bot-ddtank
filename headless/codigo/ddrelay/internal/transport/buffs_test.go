package transport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/packets"
	"ddbot/relay/internal/protocol"
)

func TestSoloBuffsThroughRemoteBot(t *testing.T) {
	path := os.Getenv("DDTANK_SOLO_SCRIPT")
	if path == "" {
		t.Skip("external solo script not configured")
	}

	if testing.Short() {
		t.Skip("requires bot process")
	}

	library, err := filepath.Abs("../../../bot/lua")
	if err != nil {
		t.Fatal(err)
	}

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	cc, ctx := startBot(t, library, func(ctx context.Context, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", address)
	})
	stream, err := pb.NewBotClient(cc).GameSession(metadata.AppendToOutgoingContext(ctx, "x-api-key", "synthetic-test-api-key"))
	if err != nil {
		t.Fatal(err)
	}

	if err := stream.Send(&pb.RelayMessage{Payload: &pb.RelayMessage_Open{Open: &pb.OpenSession{
		ApiVersion: contract.APIVersion, CodecRevision: contract.CodecRevision,
		RelayInstanceId: "buff-relay", GameSessionId: "buff-game", LocalStream: 1,
	}}}); err != nil {
		t.Fatal(err)
	}

	opened, err := stream.Recv()
	if err != nil || opened.GetOpened() == nil {
		t.Fatalf("open: %v", err)
	}

	binding := opened.GetOpened()
	send := func(message *pb.RelayMessage) {
		t.Helper()
		message.BotSessionId = binding.BotSessionId
		message.Generation = binding.Generation
		if err := stream.Send(message); err != nil {
			t.Fatal(err)
		}
	}
	sequence := uint64(0)
	direction := capture.ServerToClient
	rejectLoading := false
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	event := func(code int16, fields map[string]any) []framing.Frame {
		t.Helper()
		sequence++
		packet, err := protocol.Event(sequence, capture.Record{Time: now, Direction: direction},
			framing.Header{Code: code, ClientID: 7}, packets.Result{Status: "decoded", Fields: fields})
		if err != nil {
			t.Fatal(err)
		}

		send(&pb.RelayMessage{Payload: &pb.RelayMessage_Packet{Packet: packet}})
		var frames []framing.Frame
		for {
			message, err := stream.Recv()
			if err != nil {
				t.Fatal(err)
			}

			if message.GetApplied().GetSequence() == sequence {
				return frames
			}

			batch := message.GetBatch()
			if batch == nil {
				t.Fatal("unexpected bot response")
			}

			for _, command := range batch.Commands {
				frame, err := protocol.Encode(command)
				if err != nil {
					t.Fatal(err)
				}

				decoded := packets.NewDecoder().Decode(capture.ClientToServer, frame)
				if decoded.Status != "decoded" || decoded.Consumed != len(frame.Body) {
					t.Fatal("command did not decode")
				}

				frames = append(frames, frame)
				receipt := &pb.CommandReceipt{
					BatchId: batch.Id, CommandId: command.Id, Status: pb.ReceiptStatus_RECEIPT_STATUS_SENT,
					BytesWritten: uint32(20 + len(frame.Body)),
				}
				if rejectLoading && decoded.Fields["subcommand"] == uint8(16) {
					receipt.Status = pb.ReceiptStatus_RECEIPT_STATUS_REJECTED
					receipt.Error = &pb.Failure{Code: pb.FailureCode_FAILURE_CODE_STALE_STATE, Message: "stale_state"}
					receipt.BytesWritten = 0
				}

				send(&pb.RelayMessage{Payload: &pb.RelayMessage_Receipt{Receipt: receipt}})
			}
		}
	}
	event(1, map[string]any{"status": int32(0)})
	event(64, map[string]any{"bag_type": int32(3), "slots": []map[string]any{
		{"place": int32(1), "present": true, "item": map[string]any{
			"template_id": int32(10004), "count": int32(1), "valid_date": int32(0), "begin_date": "2026-10-01T00:00:00", "used": false,
		}},
	}})
	event(91, map[string]any{"subcommand": uint8(101), "players": []map[string]any{
		{"living_id": int32(0), "player_id": int32(7), "nickname": "Player", "team": int32(1), "max_hp": int32(1000), "weapon_id": int32(7001)},
	}})
	event(91, map[string]any{"subcommand": uint8(103), "map_id": int32(1120)})
	self := map[string]any{"living_id": int32(0), "x": int32(680), "y": int32(504), "hp": int32(1000), "max_hp": int32(1000), "max_energy": int32(100), "dander": int32(200), "properties": []map[string]any{}, "frozen": false}
	event(91, map[string]any{"subcommand": uint8(99), "players": []map[string]any{self}})
	event(91, map[string]any{"subcommand": uint8(64), "living_id": int32(1), "x": int32(253), "y": int32(507), "hp": int32(500), "max_hp": int32(500), "team": int32(2)})
	for id, glyph := range []string{"0", "."} {
		event(91, map[string]any{"subcommand": uint8(241), "picture_id": int8(id), "picture_sha256": "synthetic", "picture_glyph": glyph})
	}

	hash := sha256.Sum256(source)
	send(&pb.RelayMessage{Payload: &pb.RelayMessage_Load{Load: &pb.LoadScript{RequestId: "solo-buffs", Source: string(source), Sha256: hash[:]}}})
	loaded, err := stream.Recv()
	if err != nil || loaded.GetResult() == nil || loaded.GetResult().GetError() != nil {
		t.Fatalf("load: %v %v", loaded, err)
	}

	turn := map[string]any{"subcommand": uint8(6), "turn_number": int32(1), "turn_time": int32(30), "wind_display_flag": true, "wind_display_bytes": []int8{0, 1, 0}, "livings": []map[string]any{self}}
	frames := event(91, turn)
	if len(frames) != 1 || !bytes.Equal(frames[0].Body, []byte{15, 0}) {
		t.Fatalf("POW frames=%v", frames)
	}

	if frames := event(4, map[string]any{}); len(frames) != 0 {
		t.Fatal("shot before POW acceptance")
	}

	frames = event(91, map[string]any{"subcommand": uint8(20), "living_id": int32(0), "special": true, "ball_id": int32(21)})
	if len(frames) != 1 || !bytes.Equal(frames[0].Body, wireJoin([]byte{32, 1}, wireInts(-1, 10004))) {
		t.Fatalf("prop frames=%v", frames)
	}

	if frames := event(4, map[string]any{}); len(frames) != 0 {
		t.Fatal("shot before prop acceptance")
	}

	frames = event(91, map[string]any{"subcommand": uint8(32), "living_id": int32(0), "bag_type": int8(-2), "place": int32(-2), "template_id": int32(10004)})
	assertSoloShot(t, frames)

	self["dander"] = int32(0)
	turn["turn_number"] = int32(2)
	frames = event(91, turn)
	if len(frames) != 1 || !bytes.Equal(frames[0].Body, wireJoin([]byte{32, 1}, wireInts(-1, 10004))) {
		t.Fatal("free percentage prop not requested on next turn")
	}
	frames = event(91, map[string]any{"subcommand": uint8(32), "living_id": int32(0), "bag_type": int8(-2), "place": int32(-2), "template_id": int32(10004)})
	assertSoloShot(t, frames)
	event(91, map[string]any{"subcommand": uint8(46), "shoot_count": int8(2)})
	fire := map[string]any{"subcommand": uint8(2), "living_id": int32(0), "wind_tenths": int32(0), "wind_display_flag": true, "wind_display_bytes": []int8{0, 1, 0}, "projectiles": []map[string]any{}, "pet_targets": []map[string]any{}}
	if got := event(91, fire); len(got) != 0 {
		t.Fatal("additional shot sent before its interval")
	}
	now = now.Add(time.Second)
	extra := event(4, map[string]any{})
	if len(extra) != 1 || !bytes.Equal(extra[0].Body, frames[2].Body) {
		t.Fatal("additional shot not sent with the same trajectory")
	}
	now = now.Add(time.Second)
	if got := event(91, fire); len(got) != 0 {
		t.Fatal("additional shot duplicated")
	}

	rejectLoading = true
	load := wireJoin([]byte{16}, wireInts(100))
	frames = event(91, map[string]any{"subcommand": uint8(103), "map_id": int32(1121)})
	if len(frames) != 1 || !bytes.Equal(frames[0].Body, load) {
		t.Fatal("boss loading progress not requested")
	}
	if got := event(91, map[string]any{"subcommand": uint8(241), "picture_id": int8(0), "picture_sha256": "synthetic", "picture_glyph": "0"}); len(got) != 0 {
		t.Fatal("loading retried before a heartbeat")
	}

	rejectLoading = false
	direction = capture.ClientToServer
	frames = event(4, map[string]any{})
	if len(frames) != 1 || !bytes.Equal(frames[0].Body, load) {
		send(&pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "loading-status", Source: "return formigueiro_status()"}}})
		result, err := stream.Recv()
		if err != nil {
			t.Fatal(err)
		}
		status, err := contract.DecodeValue(result.GetResult().Value)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("stale loading request stalled the boss stage: %v", status)
	}
	direction = capture.ServerToClient
	if got := event(4, map[string]any{}); len(got) != 0 {
		t.Fatal("sent loading request duplicated")
	}
	event(91, map[string]any{"subcommand": uint8(99), "players": []map[string]any{self}})
	event(91, map[string]any{"subcommand": uint8(64), "living_id": int32(1), "x": int32(253), "y": int32(507), "hp": int32(3500), "max_hp": int32(3500), "team": int32(2)})
	turn["turn_number"] = int32(1)
	frames = event(91, turn)
	if len(frames) != 1 || !bytes.Equal(frames[0].Body, wireJoin([]byte{32, 1}, wireInts(-1, 10004))) {
		t.Fatal("boss combat did not resume after loading")
	}
	frames = event(91, map[string]any{"subcommand": uint8(32), "living_id": int32(0), "bag_type": int8(-2), "place": int32(-2), "template_id": int32(10004)})
	if len(frames) != 3 || frames[2].Header.Code != 91 || frames[2].Body[0] != 2 {
		t.Fatal("boss shot not emitted after loading recovery and buff acceptance")
	}
}

func assertSoloShot(t *testing.T, frames []framing.Frame) {
	t.Helper()
	expected := [][]byte{
		wireJoin([]byte{7}, wireInts(-1)),
		{96, 1, 0},
		wireJoin([]byte{2}, wireInts(648, 483, 573, -140)),
	}
	if len(frames) != len(expected) {
		t.Fatalf("shot frame count=%d", len(frames))
	}

	for i, body := range expected {
		if frames[i].Header.Code != 91 || !bytes.Equal(frames[i].Body, body) {
			t.Fatalf("shot frame %d differs: %x", i, frames[i].Body)
		}
	}
}
