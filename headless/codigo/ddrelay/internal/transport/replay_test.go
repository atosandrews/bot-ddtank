package transport

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
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

func TestPrivateCapturePhysicsAcrossGRPC(t *testing.T) {
	path := os.Getenv("DDTANK_REPLAY_CAPTURE")
	if path == "" {
		t.Skip("set DDTANK_REPLAY_CAPTURE to the private cooperative recording")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	before := sha256.Sum256(raw)
	if fmt.Sprintf("%x", before) != "05464fd8d0ed21ed72c9412b0048f180be9a70862d08e87fe5024d8b23bf26e3" {
		t.Fatal("expected the documented cooperative recording")
	}

	library, err := filepath.Abs("../../../bot/lua")
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
		ApiVersion:      contract.APIVersion,
		CodecRevision:   contract.CodecRevision,
		RelayInstanceId: "replay",
		GameSessionId:   "game",
		LocalStream:     2,
	}}}); err != nil {
		t.Fatal(err)
	}

	opened, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
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
	source, err := os.ReadFile("testdata/physics.lua")
	if err != nil {
		t.Fatal(err)
	}

	hash := sha256.Sum256(source)
	send(&pb.RelayMessage{Payload: &pb.RelayMessage_Load{Load: &pb.LoadScript{RequestId: "physics", Source: string(source), Sha256: hash[:]}}})
	result, err := stream.Recv()
	if err != nil || result.GetResult().GetError() != nil {
		t.Fatalf("load failed %v %v", result, err)
	}

	framers := map[capture.Direction]*framing.EncryptedFramer{capture.ClientToServer: framing.NewEncryptedFramer(), capture.ServerToClient: framing.NewEncryptedFramer()}
	sequence := uint64(0)
	var socialValue any
	counts := map[string]int{}
	decoder := packets.NewDecoder()
	err = capture.Read(path, func(record capture.Record) error {
		if record.Stream != 2 {
			return nil
		}

		if record.Lost != 0 {
			return fmt.Errorf("capture has a gap")
		}

		framer, ok := framers[record.Direction]
		if !ok {
			return fmt.Errorf("unknown direction")
		}

		for _, segment := range framer.Push(record.Data) {
			if segment.Kind != framing.FrameSegment || !segment.Decrypted {
				continue
			}

			decoded := decoder.Decode(record.Direction, segment.Frame)
			counts[decoded.Status]++
			sequence++
			event, err := protocol.Event(sequence, record, segment.Frame.Header, decoded)
			if err != nil {
				return err
			}

			send(&pb.RelayMessage{Payload: &pb.RelayMessage_Packet{Packet: event}})
			answer, err := stream.Recv()
			if err != nil {
				return err
			}

			if answer.GetApplied().GetSequence() != sequence {
				return fmt.Errorf("expected applied sequence %d", sequence)
			}

			if record.Direction == capture.ServerToClient && segment.Frame.Header.Code == 91 && decoded.Fields["subcommand"] == uint8(89) {
				send(&pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: fmt.Sprintf("social-%d", sequence), Source: `local f=friends.state()
local c=fight.cards()
assert(f.last_result and f.last_result.success)
assert(c.known and c.type==2 and c.closed and c.finished)
return {friends=#f.players,awards=#c.awards,revealed=#c.revealed,requests=#c.requested}
`}}})
				result, err := stream.Recv()
				if err != nil || result.GetResult().GetError() != nil {
					return fmt.Errorf("private social/reward audit failed at sequence %d", sequence)
				}

				socialValue, err = contract.DecodeValue(result.GetResult().Value)
				if err != nil {
					return err
				}
			}

		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if sequence != 2137 || !reflect.DeepEqual(counts, map[string]int{"decoded": 2066, "partial": 37, "unknown": 34}) {
		t.Fatalf("coverage %d %v", sequence, counts)
	}

	socialWant := map[string]any{
		"friends":  float64(1),
		"awards":   float64(5),
		"revealed": float64(16),
		"requests": float64(5),
	}
	if !reflect.DeepEqual(socialValue, socialWant) {
		t.Fatalf("private audit counts=%v want=%v", socialValue, socialWant)
	}

	send(&pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "audit", Source: `return audit_result()`}}})
	result, err = stream.Recv()
	if err != nil || result.GetResult().GetError() != nil {
		t.Fatalf("audit failed %v %v", result, err)
	}

	value, err := contract.DecodeValue(result.GetResult().Value)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]any{
		"origins":    float64(8),
		"velocities": float64(8),
		"own_turns":  float64(8),
		"found":      float64(1),
		"cancelled":  float64(5),
	}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("physics audit %v", value)
	}

	catalogStart := time.Now()
	send(&pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "items", Source: `local all=require("data.items");local counts={total=0};for _,group in ipairs({"resources","equipments","skins"}) do local n=0;for id,item in pairs(require("data.items."..group)) do assert(all[id]==item);n=n+1 end;counts[group]=n;counts.total=counts.total+n end;return counts`}}})
	result, err = stream.Recv()
	if err != nil || result.GetResult().GetError() != nil {
		t.Fatalf("catalog failed %v %v", result, err)
	}

	value, err = contract.DecodeValue(result.GetResult().Value)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(value, map[string]any{
		"total":      float64(13076),
		"resources":  float64(6252),
		"equipments": float64(4210),
		"skins":      float64(2614),
	}) {
		t.Fatalf("catalog count %v", value)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if sha256.Sum256(after) != before {
		t.Fatal("recording changed")
	}

	t.Logf("2137 packets, 8 origins, 8 velocities, 13076 items; capture unchanged; first full catalog evaluation %s", time.Since(catalogStart))
}
