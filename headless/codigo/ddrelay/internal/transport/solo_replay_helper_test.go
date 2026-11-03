package transport

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc/metadata"

	"ddbot/api/contract"
	pb "ddbot/api/gen/go/ddbot/v1"
	"ddbot/relay/internal/capture"
	"ddbot/relay/internal/framing"
	"ddbot/relay/internal/packets"
	"ddbot/relay/internal/protocol"
)

func replaySoloScript(t *testing.T, path, expectedHash, auditPath string) soloReplayReport {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	before := sha256.Sum256(raw)
	if fmt.Sprintf("%x", before) != expectedHash {
		t.Fatal("expected the documented solo failure recording")
	}

	library, err := filepath.Abs("../../../bot/lua")
	if err != nil {
		t.Fatal(err)
	}

	scriptPath := os.Getenv("DDTANK_SOLO_SCRIPT")
	if scriptPath == "" {
		t.Fatal("DDTANK_SOLO_SCRIPT is required with DDTANK_SOLO_CAPTURE")
	}

	source, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}

	auditSource, err := os.ReadFile(auditPath)
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
		RelayInstanceId: "solo-replay", GameSessionId: "game", LocalStream: 2,
	}}}); err != nil {
		t.Fatal(err)
	}

	opened, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}

	binding := opened.GetOpened()
	send := func(message *pb.RelayMessage) error {
		message.BotSessionId = binding.BotSessionId
		message.Generation = binding.Generation
		return stream.Send(message)
	}

	framers := map[capture.Direction]*framing.EncryptedFramer{
		capture.ClientToServer: framing.NewEncryptedFramer(), capture.ServerToClient: framing.NewEncryptedFramer(),
	}
	sequence := uint64(0)
	shots := 0
	props := 0
	loaded := false
	decoder := packets.NewDecoder()
	err = capture.Read(path, func(record capture.Record) error {
		if record.Stream != 2 {
			return nil
		}

		if record.Lost != 0 {
			return fmt.Errorf("capture contains a gap")
		}

		for _, segment := range framers[record.Direction].Push(record.Data) {
			if segment.Kind != framing.FrameSegment || !segment.Decrypted {
				continue
			}

			decoded := decoder.Decode(record.Direction, segment.Frame)
			sequence++
			event, err := protocol.Event(sequence, record, segment.Frame.Header, decoded)
			if err != nil {
				return err
			}

			if err := send(&pb.RelayMessage{Payload: &pb.RelayMessage_Packet{Packet: event}}); err != nil {
				return err
			}

			for {
				answer, err := stream.Recv()
				if err != nil {
					return err
				}

				if answer.GetApplied().GetSequence() == sequence {
					break
				}

				batch := answer.GetBatch()
				if batch == nil {
					return fmt.Errorf("unexpected replay response at sequence %d", sequence)
				}

				for _, command := range batch.Commands {
					if command.GetShoot() != nil {
						shots++
					}
					if command.GetUseProp() != nil {
						props++
					}

					frame, err := protocol.Encode(command)
					if err != nil {
						return err
					}

					if err := send(&pb.RelayMessage{Payload: &pb.RelayMessage_Receipt{Receipt: &pb.CommandReceipt{
						BatchId: batch.Id, CommandId: command.Id, Status: pb.ReceiptStatus_RECEIPT_STATUS_SENT,
						BytesWritten: uint32(20 + len(frame.Body)),
					}}}); err != nil {
						return err
					}
				}
			}

			if !loaded && record.Direction == capture.ServerToClient && segment.Frame.Header.Code == 91 && decoded.Fields["subcommand"] == uint8(99) {
				wrapped := "local hooks=(function()\n" + string(source) + "\nend)()\n" + string(auditSource)
				hash := sha256.Sum256([]byte(wrapped))
				if err := send(&pb.RelayMessage{Payload: &pb.RelayMessage_Load{Load: &pb.LoadScript{RequestId: "solo", Source: wrapped, Sha256: hash[:]}}}); err != nil {
					return err
				}

				result, err := stream.Recv()
				if err != nil || result.GetResult().GetError() != nil {
					return fmt.Errorf("solo script load failed")
				}

				loaded = true
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := send(&pb.RelayMessage{Payload: &pb.RelayMessage_Evaluate{Evaluate: &pb.Evaluate{RequestId: "audit", Source: "return solo_audit()"}}}); err != nil {
		t.Fatal(err)
	}

	result, err := stream.Recv()
	if err != nil || result.GetResult().GetError() != nil {
		t.Fatal("audit failed")
	}

	audit, err := contract.DecodeValue(result.GetResult().Value)
	if err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(path)
	if err != nil || sha256.Sum256(after) != before {
		t.Fatal("capture changed")
	}

	report, ok := audit.(map[string]any)
	if !ok {
		t.Fatal("audit is not a table")
	}
	return soloReplayReport{frames: sequence, shots: shots, props: props, audit: report}
}
